package display

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/Kishanmp3/breaklog/internal/db"
)

// Colour palette.
var (
	accent  = lipgloss.NewStyle().Foreground(lipgloss.Color("99"))  // indigo
	muted   = lipgloss.NewStyle().Foreground(lipgloss.Color("241")) // dark gray
	bold    = lipgloss.NewStyle().Bold(true)
	green   = lipgloss.NewStyle().Foreground(lipgloss.Color("83"))
	dim     = lipgloss.NewStyle().Faint(true)
	divider = muted.Render(strings.Repeat("─", 44))
)

// PrintSessionClosed prints a compact summary when the daemon closes a session.
func PrintSessionClosed(s *db.Session) {
	fmt.Println()
	fmt.Printf("%s  %s  %s\n",
		accent.Render("●"),
		bold.Render(s.ProjectName),
		muted.Render("session closed"),
	)
	if s.DurationSec != nil {
		fmt.Printf("  %s  %s\n",
			dim.Render("duration"),
			db.FormatDuration(*s.DurationSec),
		)
	}
	if s.Summary != "" {
		wrapped := wrapText(s.Summary, 60)
		for _, line := range wrapped {
			fmt.Printf("  %s\n", line)
		}
	}
	fmt.Println()
}

// ShowToday renders today's sessions in the style shown in the spec.
func ShowToday(sessions []*db.Session) {
	if len(sessions) == 0 {
		fmt.Println(muted.Render("No sessions today."))
		return
	}

	header := bold.Render(time.Now().Format("Monday, Jan 2"))
	fmt.Println(header)
	fmt.Println(divider)

	var totalSec int64
	for _, s := range sessions {
		printSessionRow(s)
		if s.DurationSec != nil {
			totalSec += *s.DurationSec
		}
	}

	fmt.Println(divider)
	fmt.Printf("Total: %s\n", bold.Render(db.FormatDuration(totalSec)))
}

// ShowWeeklySummary renders the rolled-up weekly summary.
func ShowWeeklySummary(sessions []*db.Session, rollup string) {
	if len(sessions) == 0 {
		fmt.Println(muted.Render("No completed sessions this week."))
		return
	}

	// Calculate stats.
	var totalSec int64
	projectCounts := map[string]int{}
	for _, s := range sessions {
		if s.DurationSec != nil {
			totalSec += *s.DurationSec
		}
		projectCounts[s.ProjectName]++
	}

	// Header.
	weekStart := time.Now().AddDate(0, 0, -7)
	fmt.Println(bold.Render("Week of " + weekStart.Format("Jan 2")))
	fmt.Println(divider)

	if rollup != "" {
		fmt.Println(bold.Render("You shipped:"))
		for _, line := range strings.Split(rollup, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			// Ensure bullet formatting.
			if !strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "•") {
				line = "  - " + line
			} else {
				line = "  " + line
			}
			fmt.Println(line)
		}
		fmt.Println()
	}

	// Footer stats.
	projects := make([]string, 0, len(projectCounts))
	for p := range projectCounts {
		projects = append(projects, p)
	}
	fmt.Printf("%s · %s · %s\n",
		muted.Render(fmt.Sprintf("%d sessions", len(sessions))),
		muted.Render(db.FormatDuration(totalSec)),
		muted.Render(strings.Join(projects, ", ")),
	)
}

// ShowStatus renders the current session status.
func ShowStatus(sessions []*db.Session) {
	if len(sessions) == 0 {
		fmt.Println(muted.Render("No active session."))
		return
	}
	for _, s := range sessions {
		elapsed := time.Since(s.StartedAt)
		minutes := int(elapsed.Minutes())
		fmt.Printf("%s Active session on %s (started %dmin ago)\n",
			green.Render("→"),
			bold.Render(s.ProjectName),
			minutes,
		)
		if len(s.FilesChanged) > 0 {
			fmt.Printf("   Files changed so far: %s\n",
				dim.Render(strings.Join(s.FilesChanged, ", ")),
			)
		}
	}
}

// ShowWatching prints the confirmation message after registering a project.
func ShowWatching(path string) {
	fmt.Printf("%s Now watching: %s\n", green.Render("→"), bold.Render(path))
}

// ShowProjects lists all watched projects.
func ShowProjects(projects []*db.Project) {
	if len(projects) == 0 {
		fmt.Println(muted.Render("No projects registered. Run: daimon haunt [path]"))
		return
	}
	fmt.Println(bold.Render("Watched projects:"))
	for _, p := range projects {
		fmt.Printf("  %s  %s\n", accent.Render("●"), p.Path)
	}
}

// ──────────────────────────────────────────────────────────────────
// Internal helpers
// ──────────────────────────────────────────────────────────────────

func printSessionRow(s *db.Session) {
	timeStr := s.StartedAt.Local().Format("15:04")
	dur := ""
	if s.DurationSec != nil {
		dur = db.FormatDuration(*s.DurationSec)
	} else {
		// Active session — show elapsed.
		elapsed := int64(time.Since(s.StartedAt).Seconds())
		dur = db.FormatDuration(elapsed) + " (active)"
	}

	fmt.Printf("%s  %-14s  %s\n",
		muted.Render(timeStr),
		bold.Render(s.ProjectName),
		dur,
	)

	if s.Summary != "" {
		for _, line := range wrapText(s.Summary, 56) {
			fmt.Printf("       %s\n", line)
		}
	}
	fmt.Println()
}

// wrapText wraps a string to maxWidth characters per line.
func wrapText(s string, maxWidth int) []string {
	words := strings.Fields(s)
	var lines []string
	var current strings.Builder

	for _, w := range words {
		if current.Len() > 0 && current.Len()+1+len(w) > maxWidth {
			lines = append(lines, current.String())
			current.Reset()
		}
		if current.Len() > 0 {
			current.WriteByte(' ')
		}
		current.WriteString(w)
	}
	if current.Len() > 0 {
		lines = append(lines, current.String())
	}
	return lines
}
