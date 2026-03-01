package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"
)

// Project represents a watched project directory.
type Project struct {
	ID         int64
	Name       string
	Path       string
	ShadowRepo string
	CreatedAt  time.Time
}

// Session represents a single coding session.
type Session struct {
	ID           int64
	ProjectID    int64
	ProjectName  string
	ProjectPath  string
	StartedAt    time.Time
	EndedAt      *time.Time
	DurationSec  *int64
	Status       string
	RawDiff      string
	Summary      string
	FilesChanged []string
	SnapshotHash string
}

// ──────────────────────────────────────────────────────────────────
// Projects
// ──────────────────────────────────────────────────────────────────

// UpsertProject inserts or updates a project record.
func (db *DB) UpsertProject(path, name, shadowRepo string) (*Project, error) {
	_, err := db.Exec(`
		INSERT INTO projects (name, path, shadow_repo) VALUES (?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET name=excluded.name, shadow_repo=excluded.shadow_repo
	`, name, path, shadowRepo)
	if err != nil {
		return nil, fmt.Errorf("upsert project: %w", err)
	}
	return db.GetProjectByPath(path)
}

// GetProjectByPath returns the project for the given directory path, or nil if not found.
func (db *DB) GetProjectByPath(path string) (*Project, error) {
	row := db.QueryRow(
		`SELECT id, name, path, shadow_repo, created_at FROM projects WHERE path = ?`, path,
	)
	return scanProject(row)
}

// GetAllProjects returns all registered projects.
func (db *DB) GetAllProjects() ([]*Project, error) {
	rows, err := db.Query(`SELECT id, name, path, shadow_repo, created_at FROM projects ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []*Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, nil
}

// ──────────────────────────────────────────────────────────────────
// Sessions
// ──────────────────────────────────────────────────────────────────

// CreateSession opens a new active session for the given project.
func (db *DB) CreateSession(projectID int64, snapshotHash string) (*Session, error) {
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	result, err := db.Exec(`
		INSERT INTO sessions (project_id, started_at, status, snapshot_hash)
		VALUES (?, ?, 'active', ?)
	`, projectID, now, snapshotHash)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	id, _ := result.LastInsertId()
	return db.GetSessionByID(id)
}

// GetSessionByID fetches a session by its primary key.
func (db *DB) GetSessionByID(id int64) (*Session, error) {
	row := db.QueryRow(sessionSelectSQL+` WHERE s.id = ?`, id)
	return scanSession(row)
}

// GetActiveSessionForProject returns the current active session for a project, or nil.
func (db *DB) GetActiveSessionForProject(projectID int64) (*Session, error) {
	row := db.QueryRow(
		sessionSelectSQL+` WHERE s.project_id = ? AND s.status = 'active' ORDER BY s.started_at DESC LIMIT 1`,
		projectID,
	)
	return scanSession(row)
}

// GetAllActiveSessions returns all currently active sessions across all projects.
func (db *DB) GetAllActiveSessions() ([]*Session, error) {
	rows, err := db.Query(sessionSelectSQL + ` WHERE s.status = 'active' ORDER BY s.started_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSessions(rows)
}

// GetSessionsForToday returns all sessions that started today.
func (db *DB) GetSessionsForToday() ([]*Session, error) {
	today := time.Now().Format("2006-01-02")
	rows, err := db.Query(
		sessionSelectSQL+` WHERE DATE(s.started_at) = ? ORDER BY s.started_at ASC`,
		today,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSessions(rows)
}

// GetSessionsForWeek returns all closed sessions from the last 7 days.
func (db *DB) GetSessionsForWeek() ([]*Session, error) {
	weekAgo := time.Now().AddDate(0, 0, -7).UTC().Format("2006-01-02 15:04:05")
	rows, err := db.Query(
		sessionSelectSQL+` WHERE s.started_at >= ? AND s.status = 'closed' ORDER BY s.started_at ASC`,
		weekAgo,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSessions(rows)
}

// UpdateSessionFiles updates the files_changed column for a live session.
func (db *DB) UpdateSessionFiles(sessionID int64, files []string) error {
	data, _ := json.Marshal(files)
	_, err := db.Exec(`UPDATE sessions SET files_changed = ? WHERE id = ?`, string(data), sessionID)
	return err
}

// CloseSession marks a session as closed and stores the diff, summary, and file list.
func (db *DB) CloseSession(sessionID int64, endedAt, startedAt time.Time, diff, summary string, files []string) error {
	filesData, _ := json.Marshal(files)
	var duration int64
	if !startedAt.IsZero() && endedAt.After(startedAt) {
		duration = int64(endedAt.Sub(startedAt).Seconds())
	}
	_, err := db.Exec(`
		UPDATE sessions SET
			status       = 'closed',
			ended_at     = ?,
			duration_sec = ?,
			raw_diff     = ?,
			summary      = ?,
			files_changed = ?
		WHERE id = ?
	`,
		endedAt.UTC().Format("2006-01-02 15:04:05"),
		duration,
		diff,
		summary,
		string(filesData),
		sessionID,
	)
	return err
}

// ──────────────────────────────────────────────────────────────────
// Config
// ──────────────────────────────────────────────────────────────────

// SetConfig stores a key-value config entry.
func (db *DB) SetConfig(key, value string) error {
	_, err := db.Exec(
		`INSERT INTO config (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
		key, value,
	)
	return err
}

// GetConfig retrieves a config value by key. Returns ("", nil) if not set.
func (db *DB) GetConfig(key string) (string, error) {
	var value string
	err := db.QueryRow(`SELECT value FROM config WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// ──────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────

const sessionSelectSQL = `
	SELECT s.id, s.project_id, p.name, p.path, s.started_at,
	       s.ended_at, s.duration_sec, s.status,
	       COALESCE(s.raw_diff,''), COALESCE(s.summary,''),
	       COALESCE(s.files_changed,'[]'), COALESCE(s.snapshot_hash,'')
	FROM sessions s
	JOIN projects p ON s.project_id = p.id`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanProject(row rowScanner) (*Project, error) {
	p := &Project{}
	var createdAt string
	err := row.Scan(&p.ID, &p.Name, &p.Path, &p.ShadowRepo, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	return p, nil
}

var timeFormats = []string{
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05Z",
	time.RFC3339,
	time.RFC3339Nano,
}

func parseTime(s string) time.Time {
	for _, f := range timeFormats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func scanSession(row rowScanner) (*Session, error) {
	s := &Session{}
	var startedAt string
	var endedAt *string
	var filesJSON string
	err := row.Scan(
		&s.ID, &s.ProjectID, &s.ProjectName, &s.ProjectPath,
		&startedAt, &endedAt, &s.DurationSec,
		&s.Status, &s.RawDiff, &s.Summary, &filesJSON, &s.SnapshotHash,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s.StartedAt = parseTime(startedAt)
	if endedAt != nil {
		t := parseTime(*endedAt)
		s.EndedAt = &t
	}
	_ = json.Unmarshal([]byte(filesJSON), &s.FilesChanged)
	return s, nil
}

func scanSessions(rows *sql.Rows) ([]*Session, error) {
	var sessions []*Session
	for rows.Next() {
		s := &Session{}
		var startedAt string
		var endedAt *string
		var filesJSON string
		err := rows.Scan(
			&s.ID, &s.ProjectID, &s.ProjectName, &s.ProjectPath,
			&startedAt, &endedAt, &s.DurationSec,
			&s.Status, &s.RawDiff, &s.Summary, &filesJSON, &s.SnapshotHash,
		)
		if err != nil {
			return nil, err
		}
		s.StartedAt = parseTime(startedAt)
		if endedAt != nil {
			t := parseTime(*endedAt)
			s.EndedAt = &t
		}
		_ = json.Unmarshal([]byte(filesJSON), &s.FilesChanged)
		sessions = append(sessions, s)
	}
	return sessions, nil
}

// ProjectNameFromPath derives a display name from an absolute directory path.
func ProjectNameFromPath(path string) string {
	return filepath.Base(path)
}

// FormatDuration converts seconds into a "Xh Ym" string.
func FormatDuration(sec int64) string {
	h := sec / 3600
	m := (sec % 3600) / 60
	if h == 0 {
		return fmt.Sprintf("0h %02dm", m)
	}
	return fmt.Sprintf("%dh %02dm", h, m)
}
