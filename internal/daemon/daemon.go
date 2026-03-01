package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/Kishanmp3/breaklog/internal/db"
	"github.com/Kishanmp3/breaklog/internal/display"
	sess "github.com/Kishanmp3/breaklog/internal/session"
)

const idleTimeout = 30 * time.Minute

// projectState tracks the live state for one watched project.
type projectState struct {
	project      *db.Project
	sessionID    *int64
	sessionStart time.Time
	lastActivity time.Time
	filesChanged map[string]bool
	mu           sync.Mutex
}

func (ps *projectState) touch(file string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.lastActivity = time.Now()
	if file != "" {
		ps.filesChanged[file] = true
	}
}

func (ps *projectState) isIdle() bool {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return time.Since(ps.lastActivity) >= idleTimeout
}

func (ps *projectState) hasSession() bool {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return ps.sessionID != nil
}

// Daemon manages file watchers and session lifecycles.
type Daemon struct {
	database *db.DB
	states   map[int64]*projectState // project.ID -> state
	mu       sync.Mutex
}

// New creates a Daemon backed by the given database connection.
func New(database *db.DB) *Daemon {
	return &Daemon{
		database: database,
		states:   make(map[int64]*projectState),
	}
}

// Run starts the daemon. It blocks until the process is killed.
func (d *Daemon) Run() error {
	if err := writePID(); err != nil {
		return err
	}
	defer os.Remove(db.PIDFile())

	projects, err := d.database.GetAllProjects()
	if err != nil {
		return fmt.Errorf("load projects: %w", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}
	defer watcher.Close()

	// Set up per-project state and watches.
	for _, p := range projects {
		ps := &projectState{
			project:      p,
			filesChanged: make(map[string]bool),
		}
		d.states[p.ID] = ps

		if err := addWatchRecursive(watcher, p.Path); err != nil {
			fmt.Fprintf(os.Stderr, "warning: cannot watch %s: %v\n", p.Path, err)
		}
	}

	// Supplement from projects.json — picks up projects added while daemon was stopped.
	for _, p := range d.loadFromProjectsJSON() {
		if _, exists := d.states[p.ID]; !exists {
			ps := &projectState{
				project:      p,
				filesChanged: make(map[string]bool),
			}
			d.states[p.ID] = ps
			if err := addWatchRecursive(watcher, p.Path); err != nil {
				fmt.Fprintf(os.Stderr, "warning: cannot watch %s: %v\n", p.Path, err)
			}
		}
	}

	n := len(d.states)
	fmt.Printf("→ daimon running. Watching %d project", n)
	if n != 1 {
		fmt.Print("s")
	}
	fmt.Println(".")

	// Idle-check ticker — runs every minute.
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				d.handleFileEvent(event.Name)
			}
			// If a new directory is created, watch it too.
			if event.Has(fsnotify.Create) {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					_ = addWatchRecursive(watcher, event.Name)
				}
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Fprintf(os.Stderr, "watcher error: %v\n", err)

		case <-ticker.C:
			d.checkIdleSessions()
		}
	}
}

// handleFileEvent routes a filesystem event to the correct project.
func (d *Daemon) handleFileEvent(path string) {
	if isIgnored(path) {
		return
	}

	d.mu.Lock()
	ps := d.findProject(path)
	d.mu.Unlock()

	if ps == nil {
		return
	}

	relPath := strings.TrimPrefix(filepath.ToSlash(path), filepath.ToSlash(ps.project.Path)+"/")
	ps.touch(relPath)

	if !ps.hasSession() {
		d.openSession(ps)
	} else {
		// Keep files_changed column fresh for `daimon status`.
		ps.mu.Lock()
		files := fileList(ps.filesChanged)
		sid := *ps.sessionID
		ps.mu.Unlock()
		_ = d.database.UpdateSessionFiles(sid, files)
	}
}

// openSession starts a new session for the project.
func (d *Daemon) openSession(ps *projectState) {
	session, err := sess.Start(d.database, ps.project)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error starting session for %s: %v\n", ps.project.Name, err)
		return
	}
	ps.mu.Lock()
	ps.sessionID = &session.ID
	ps.sessionStart = session.StartedAt
	ps.mu.Unlock()
}

// checkIdleSessions closes any sessions that have been idle for 30+ minutes.
func (d *Daemon) checkIdleSessions() {
	d.mu.Lock()
	psList := make([]*projectState, 0, len(d.states))
	for _, ps := range d.states {
		psList = append(psList, ps)
	}
	d.mu.Unlock()

	for _, ps := range psList {
		if !ps.hasSession() || !ps.isIdle() {
			continue
		}
		d.closeSession(ps)
	}
}

// closeSession ends an active session and prints the summary.
func (d *Daemon) closeSession(ps *projectState) {
	ps.mu.Lock()
	if ps.sessionID == nil {
		ps.mu.Unlock()
		return
	}
	sid := *ps.sessionID
	ps.sessionID = nil
	ps.filesChanged = make(map[string]bool)
	ps.mu.Unlock()

	// Load the session record from the DB.
	session, err := d.database.GetSessionByID(sid)
	if err != nil || session == nil {
		return
	}

	apiKey, _ := d.database.GetConfig("anthropic_api_key")
	apiKey = getAPIKey(apiKey)

	if err := sess.Close(d.database, session, ps.project, apiKey); err != nil {
		fmt.Fprintf(os.Stderr, "error closing session: %v\n", err)
		return
	}

	// Reload to get the stored summary.
	closed, err := d.database.GetSessionByID(sid)
	if err != nil || closed == nil {
		return
	}

	// Print to stdout — visible when daemon runs in a terminal.
	display.PrintSessionClosed(closed)
}

// findProject returns the projectState whose watched path is a prefix of filePath.
// Caller must hold d.mu.
func (d *Daemon) findProject(filePath string) *projectState {
	filePath = filepath.ToSlash(filePath)
	for _, ps := range d.states {
		prefix := filepath.ToSlash(ps.project.Path)
		if strings.HasPrefix(filePath, prefix+"/") || filePath == prefix {
			return ps
		}
	}
	return nil
}

// ──────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────

func fileList(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for f := range m {
		out = append(out, f)
	}
	return out
}

func writePID() error {
	pid := strconv.Itoa(os.Getpid())
	return os.WriteFile(db.PIDFile(), []byte(pid), 0o644)
}

// getAPIKey reads from the environment first, falling back to the stored value.
func getAPIKey(stored string) string {
	if v := os.Getenv("ANTHROPIC_API_KEY"); v != "" {
		return v
	}
	return stored
}

// loadFromProjectsJSON reads ~/.daimon/projects.json and returns any DB projects
// not already present in d.states. Safe to call after d.states is populated.
func (d *Daemon) loadFromProjectsJSON() []*db.Project {
	data, err := os.ReadFile(db.ProjectsJSONPath())
	if err != nil {
		return nil
	}
	var entries []struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil
	}

	var extra []*db.Project
	for _, e := range entries {
		found := false
		d.mu.Lock()
		for _, ps := range d.states {
			if ps.project.Path == e.Path {
				found = true
				break
			}
		}
		d.mu.Unlock()
		if !found {
			p, err := d.database.GetProjectByPath(e.Path)
			if err == nil && p != nil {
				extra = append(extra, p)
			}
		}
	}
	return extra
}
