package server

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Kishanmp3/breaklog/internal/ai"
)

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// parseDiffStats counts added/removed lines in a unified diff, ignoring file headers.
func parseDiffStats(diff string) (added, removed int) {
	for _, line := range strings.Split(diff, "\n") {
		if len(line) == 0 {
			continue
		}
		if line[0] == '+' && !strings.HasPrefix(line, "+++") {
			added++
		} else if line[0] == '-' && !strings.HasPrefix(line, "---") {
			removed++
		}
	}
	return
}

// tryParseUTC tries common SQLite timestamp formats as UTC.
func tryParseUTC(s string) (time.Time, bool) {
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		time.RFC3339,
		time.RFC3339Nano,
	}
	for _, f := range formats {
		if t, err := time.ParseInLocation(f, s, time.UTC); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// ──────────────────────────────────────────────────────────────────
// GET /api/overview
// ──────────────────────────────────────────────────────────────────

type overviewResponse struct {
	HoursToday        float64 `json:"hours_today"`
	HoursThisWeek     float64 `json:"hours_this_week"`
	TotalSessions     int     `json:"total_sessions"`
	ActiveProjects    int     `json:"active_projects"`
	StreakDays        int     `json:"streak_days"`
	FilesChangedToday int     `json:"files_changed_today"`
	IsActive          bool    `json:"is_active"`
	ActiveProject     string  `json:"active_project"`
	ActiveElapsedSec  int64   `json:"active_elapsed_sec"`
}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var hoursToday float64
	_ = s.db.QueryRow(`
		SELECT COALESCE(SUM(duration_sec),0)/3600.0 FROM sessions
		WHERE DATE(started_at,'localtime')=DATE('now','localtime') AND status='closed'
	`).Scan(&hoursToday)

	var hoursThisWeek float64
	_ = s.db.QueryRow(`
		SELECT COALESCE(SUM(duration_sec),0)/3600.0 FROM sessions
		WHERE started_at >= datetime('now','-7 days') AND status='closed'
	`).Scan(&hoursThisWeek)

	var totalSessions int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE status='closed'`).Scan(&totalSessions)

	var activeProjects int
	_ = s.db.QueryRow(`
		SELECT COUNT(DISTINCT project_id) FROM sessions WHERE status='active'
	`).Scan(&activeProjects)

	filesChangedToday := s.countFilesChangedToday()

	// Active session: project name + elapsed time
	var activeProject string
	var activeElapsedSec int64
	var isActive bool
	var activeProjName sql.NullString
	var activeStartedAt sql.NullString
	_ = s.db.QueryRow(`
		SELECT p.name, s.started_at FROM sessions s
		JOIN projects p ON s.project_id=p.id
		WHERE s.status='active' ORDER BY s.started_at DESC LIMIT 1
	`).Scan(&activeProjName, &activeStartedAt)
	if activeProjName.Valid {
		isActive = true
		activeProject = activeProjName.String
		if activeStartedAt.Valid {
			if t, ok := tryParseUTC(activeStartedAt.String); ok {
				activeElapsedSec = int64(time.Since(t).Seconds())
			}
		}
	}

	writeJSON(w, overviewResponse{
		HoursToday:        hoursToday,
		HoursThisWeek:     hoursThisWeek,
		TotalSessions:     totalSessions,
		ActiveProjects:    activeProjects,
		StreakDays:        s.calculateStreak(),
		FilesChangedToday: filesChangedToday,
		IsActive:          isActive,
		ActiveProject:     activeProject,
		ActiveElapsedSec:  activeElapsedSec,
	})
}

// countFilesChangedToday parses files_changed JSON arrays from today's sessions
// and returns the count of distinct file paths.
func (s *Server) countFilesChangedToday() int {
	rows, err := s.db.Query(`
		SELECT COALESCE(files_changed,'[]') FROM sessions
		WHERE DATE(started_at,'localtime')=DATE('now','localtime') AND status='closed'
	`)
	if err != nil {
		return 0
	}
	defer rows.Close()
	seen := make(map[string]struct{})
	for rows.Next() {
		var filesJSON string
		if err := rows.Scan(&filesJSON); err != nil {
			continue
		}
		var files []string
		_ = json.Unmarshal([]byte(filesJSON), &files)
		for _, f := range files {
			seen[f] = struct{}{}
		}
	}
	return len(seen)
}

// calculateStreak counts consecutive days with at least one closed session going back from today.
func (s *Server) calculateStreak() int {
	rows, err := s.db.Query(`
		SELECT DISTINCT DATE(started_at,'localtime') as day
		FROM sessions WHERE status='closed'
		ORDER BY day DESC
	`)
	if err != nil {
		return 0
	}
	defer rows.Close()

	streak := 0
	check := time.Now().In(time.Local).Truncate(24 * time.Hour)

	for rows.Next() {
		var dayStr string
		if err := rows.Scan(&dayStr); err != nil {
			break
		}
		t, err := time.ParseInLocation("2006-01-02", dayStr, time.Local)
		if err != nil {
			break
		}
		if !t.Equal(check) {
			break
		}
		streak++
		check = check.AddDate(0, 0, -1)
	}
	return streak
}

// calculateLongestStreak finds the longest consecutive coding day streak ever.
func (s *Server) calculateLongestStreak() int {
	rows, err := s.db.Query(`
		SELECT DISTINCT DATE(started_at,'localtime') as day
		FROM sessions WHERE status='closed'
		ORDER BY day ASC
	`)
	if err != nil {
		return 0
	}
	defer rows.Close()

	var days []time.Time
	for rows.Next() {
		var dayStr string
		if err := rows.Scan(&dayStr); err != nil {
			break
		}
		t, err := time.ParseInLocation("2006-01-02", dayStr, time.Local)
		if err != nil {
			break
		}
		days = append(days, t)
	}

	if len(days) == 0 {
		return 0
	}
	longest, current := 1, 1
	for i := 1; i < len(days); i++ {
		if days[i].Sub(days[i-1]) == 24*time.Hour {
			current++
			if current > longest {
				longest = current
			}
		} else {
			current = 1
		}
	}
	return longest
}

// ──────────────────────────────────────────────────────────────────
// GET /api/projects  +  GET /api/projects/:id/sessions
// ──────────────────────────────────────────────────────────────────

type projectRow struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	LastActive   *string   `json:"last_active"`
	SessionCount int       `json:"session_count"`
	TotalSec     int64     `json:"total_sec"`
	Sparkline    []float64 `json:"sparkline"`
}

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/projects")
	if path != "" && path != "/" {
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) >= 2 && parts[1] == "sessions" {
			id, err := strconv.ParseInt(parts[0], 10, 64)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid project id")
				return
			}
			s.handleProjectSessions(w, r, id)
			return
		}
	}

	rows, err := s.db.Query(`
		SELECT p.id, p.name, p.path,
		       MAX(s.started_at) as last_active,
		       COUNT(s.id) as session_count,
		       COALESCE(SUM(s.duration_sec),0) as total_sec
		FROM projects p
		LEFT JOIN sessions s ON s.project_id=p.id AND s.status='closed'
		GROUP BY p.id ORDER BY last_active DESC
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var projects []projectRow
	for rows.Next() {
		var p projectRow
		var lastActive sql.NullString
		if err := rows.Scan(&p.ID, &p.Name, &p.Path, &lastActive, &p.SessionCount, &p.TotalSec); err != nil {
			continue
		}
		if lastActive.Valid {
			p.LastActive = &lastActive.String
		}
		p.Sparkline = make([]float64, 7)
		projects = append(projects, p)
	}
	if projects == nil {
		projects = []projectRow{}
	}

	// Build a (projectID → index) lookup for fast sparkline assignment.
	projectIdx := make(map[int64]int, len(projects))
	for i, p := range projects {
		projectIdx[p.ID] = i
	}

	// Fetch daily hours per project for last 7 days.
	sparkRows, err := s.db.Query(`
		SELECT project_id, DATE(started_at,'localtime') as day, SUM(duration_sec)/3600.0 as hours
		FROM sessions
		WHERE status='closed' AND started_at >= datetime('now','-6 days','start of day')
		GROUP BY project_id, day
	`)
	if err == nil {
		defer sparkRows.Close()
		today := time.Now().In(time.Local).Truncate(24 * time.Hour)
		for sparkRows.Next() {
			var pid int64
			var dayStr string
			var hours float64
			if err := sparkRows.Scan(&pid, &dayStr, &hours); err != nil {
				continue
			}
			t, err := time.ParseInLocation("2006-01-02", dayStr, time.Local)
			if err != nil {
				continue
			}
			offset := int(today.Sub(t).Hours() / 24)
			if offset >= 0 && offset < 7 {
				if idx, ok := projectIdx[pid]; ok {
					// sparkline[0]=6 days ago … sparkline[6]=today
					projects[idx].Sparkline[6-offset] = hours
				}
			}
		}
	}

	writeJSON(w, projects)
}

// ──────────────────────────────────────────────────────────────────
// GET /api/projects/:id/sessions
// ──────────────────────────────────────────────────────────────────

type sessionRow struct {
	ID           int64    `json:"id"`
	ProjectName  string   `json:"project_name,omitempty"`
	StartedAt    string   `json:"started_at"`
	EndedAt      *string  `json:"ended_at"`
	DurationSec  *int64   `json:"duration_sec"`
	Status       string   `json:"status"`
	Summary      string   `json:"summary"`
	FilesChanged []string `json:"files_changed"`
	LinesAdded   int      `json:"lines_added"`
	LinesRemoved int      `json:"lines_removed"`
}

func (s *Server) handleProjectSessions(w http.ResponseWriter, r *http.Request, projectID int64) {
	rows, err := s.db.Query(`
		SELECT id, started_at, ended_at, duration_sec, summary,
		       COALESCE(files_changed,'[]'), COALESCE(raw_diff,'')
		FROM sessions WHERE project_id=? AND status='closed'
		ORDER BY started_at DESC
	`, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var sessions []sessionRow
	for rows.Next() {
		var sr sessionRow
		var endedAt sql.NullString
		var filesJSON, rawDiff string
		if err := rows.Scan(&sr.ID, &sr.StartedAt, &endedAt, &sr.DurationSec,
			&sr.Summary, &filesJSON, &rawDiff); err != nil {
			continue
		}
		if endedAt.Valid {
			sr.EndedAt = &endedAt.String
		}
		_ = json.Unmarshal([]byte(filesJSON), &sr.FilesChanged)
		if sr.FilesChanged == nil {
			sr.FilesChanged = []string{}
		}
		sr.LinesAdded, sr.LinesRemoved = parseDiffStats(rawDiff)
		sessions = append(sessions, sr)
	}
	if sessions == nil {
		sessions = []sessionRow{}
	}
	writeJSON(w, sessions)
}

// ──────────────────────────────────────────────────────────────────
// GET /api/sessions
// ──────────────────────────────────────────────────────────────────

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	rows, err := s.db.Query(`
		SELECT s.id, p.name as project_name, s.started_at, s.ended_at,
		       s.duration_sec, s.status, COALESCE(s.summary,''),
		       COALESCE(s.files_changed,'[]'), COALESCE(s.raw_diff,'')
		FROM sessions s JOIN projects p ON s.project_id=p.id
		ORDER BY s.started_at DESC LIMIT 200
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var sessions []sessionRow
	for rows.Next() {
		var sr sessionRow
		var endedAt sql.NullString
		var filesJSON, rawDiff string
		if err := rows.Scan(&sr.ID, &sr.ProjectName, &sr.StartedAt, &endedAt,
			&sr.DurationSec, &sr.Status, &sr.Summary, &filesJSON, &rawDiff); err != nil {
			continue
		}
		if endedAt.Valid {
			sr.EndedAt = &endedAt.String
		}
		_ = json.Unmarshal([]byte(filesJSON), &sr.FilesChanged)
		if sr.FilesChanged == nil {
			sr.FilesChanged = []string{}
		}
		sr.LinesAdded, sr.LinesRemoved = parseDiffStats(rawDiff)
		sessions = append(sessions, sr)
	}
	if sessions == nil {
		sessions = []sessionRow{}
	}
	writeJSON(w, sessions)
}

// ──────────────────────────────────────────────────────────────────
// GET /api/heatmap
// ──────────────────────────────────────────────────────────────────

type heatmapEntry struct {
	Date  string  `json:"date"`
	Hours float64 `json:"hours"`
}

func (s *Server) handleHeatmap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	rows, err := s.db.Query(`
		SELECT DATE(started_at,'localtime') as day,
		       SUM(duration_sec)/3600.0 as hours
		FROM sessions WHERE status='closed'
		  AND started_at >= datetime('now','-365 days')
		GROUP BY day ORDER BY day
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var entries []heatmapEntry
	for rows.Next() {
		var e heatmapEntry
		if err := rows.Scan(&e.Date, &e.Hours); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	if entries == nil {
		entries = []heatmapEntry{}
	}
	writeJSON(w, entries)
}

// ──────────────────────────────────────────────────────────────────
// GET /api/insights
// ──────────────────────────────────────────────────────────────────

type insightsResponse struct {
	HoursByProject []projectHours `json:"hours_by_project"`
	SessionsByHour []hourCount    `json:"sessions_by_hour"`
	HoursPerDay    []dayHours     `json:"hours_per_day"`
	StreakDays     int            `json:"streak_days"`
	LongestStreak  int            `json:"longest_streak"`
	TotalDaysCoded int            `json:"total_days_coded"`
}

type projectHours struct {
	Name  string  `json:"name"`
	Hours float64 `json:"hours"`
}

type hourCount struct {
	Hour  int `json:"hour"`
	Count int `json:"count"`
}

type dayHours struct {
	Date  string  `json:"date"`
	Hours float64 `json:"hours"`
}

func (s *Server) handleInsights(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Hours per project
	projectRows, err := s.db.Query(`
		SELECT p.name, COALESCE(SUM(s.duration_sec),0)/3600.0 as hours
		FROM projects p LEFT JOIN sessions s ON s.project_id=p.id AND s.status='closed'
		GROUP BY p.id ORDER BY hours DESC
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer projectRows.Close()

	var hoursByProject []projectHours
	for projectRows.Next() {
		var ph projectHours
		if err := projectRows.Scan(&ph.Name, &ph.Hours); err != nil {
			continue
		}
		hoursByProject = append(hoursByProject, ph)
	}
	if hoursByProject == nil {
		hoursByProject = []projectHours{}
	}

	// Sessions by hour of day
	hourRows, err := s.db.Query(`
		SELECT CAST(strftime('%H', started_at,'localtime') AS INTEGER) as hour, COUNT(*) as count
		FROM sessions WHERE status='closed' GROUP BY hour ORDER BY hour
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer hourRows.Close()

	var sessionsByHour []hourCount
	for hourRows.Next() {
		var hc hourCount
		if err := hourRows.Scan(&hc.Hour, &hc.Count); err != nil {
			continue
		}
		sessionsByHour = append(sessionsByHour, hc)
	}
	if sessionsByHour == nil {
		sessionsByHour = []hourCount{}
	}

	// Hours per day — last 30 days
	dayRows, err := s.db.Query(`
		SELECT DATE(started_at,'localtime') as day, SUM(duration_sec)/3600.0 as hours
		FROM sessions WHERE status='closed' AND started_at >= datetime('now','-30 days')
		GROUP BY day ORDER BY day
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer dayRows.Close()

	var hoursPerDay []dayHours
	for dayRows.Next() {
		var dh dayHours
		if err := dayRows.Scan(&dh.Date, &dh.Hours); err != nil {
			continue
		}
		hoursPerDay = append(hoursPerDay, dh)
	}
	if hoursPerDay == nil {
		hoursPerDay = []dayHours{}
	}

	var totalDaysCoded int
	_ = s.db.QueryRow(`
		SELECT COUNT(DISTINCT DATE(started_at,'localtime')) FROM sessions WHERE status='closed'
	`).Scan(&totalDaysCoded)

	writeJSON(w, insightsResponse{
		HoursByProject: hoursByProject,
		SessionsByHour: sessionsByHour,
		HoursPerDay:    hoursPerDay,
		StreakDays:     s.calculateStreak(),
		LongestStreak:  s.calculateLongestStreak(),
		TotalDaysCoded: totalDaysCoded,
	})
}

// ──────────────────────────────────────────────────────────────────
// GET /api/insights/weekly-narrative
// ──────────────────────────────────────────────────────────────────

func (s *Server) handleWeeklyNarrative(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	rows, err := s.db.Query(`
		SELECT COALESCE(summary,'') FROM sessions
		WHERE status='closed' AND started_at >= datetime('now','-7 days')
		ORDER BY started_at ASC
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var summaries []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			continue
		}
		trimmed := strings.TrimSpace(s)
		if trimmed != "" && !strings.HasPrefix(trimmed, "[") {
			summaries = append(summaries, trimmed)
		}
	}

	if len(summaries) == 0 {
		writeJSON(w, map[string]string{"narrative": "No sessions this week yet."})
		return
	}

	apiKeyStored, _ := s.db.GetConfig("anthropic_api_key")
	apiKey := ai.GetAPIKey(apiKeyStored)
	if apiKey == "" {
		writeJSON(w, map[string]string{"narrative": "Set an API key to generate a narrative: breaklog config set api-key sk-ant-..."})
		return
	}

	narrative, err := ai.SummarizeWeek(summaries, apiKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]string{"narrative": narrative})
}
