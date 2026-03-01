package server

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"

	"github.com/Kishanmp3/breaklog/internal/db"
)

// Server holds the database connection and serves the HTTP dashboard.
type Server struct {
	db       *db.DB
	embedFS  fs.FS // non-nil when web UI is embedded in the binary (prod builds)
}

// New creates a new Server. Pass a non-nil embedFS to serve the web UI from
// the binary itself; pass nil to fall back to finding web/dist on disk.
func New(database *db.DB, embedFS fs.FS) *Server {
	return &Server{db: database, embedFS: embedFS}
}

// Run starts the HTTP server on the given port and blocks until it exits.
func (s *Server) Run(port int) error {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/overview", s.handleOverview)
	mux.HandleFunc("/api/projects/", s.handleProjects)
	mux.HandleFunc("/api/projects", s.handleProjects)
	mux.HandleFunc("/api/sessions", s.handleSessions)
	mux.HandleFunc("/api/heatmap", s.handleHeatmap)
	mux.HandleFunc("/api/insights", s.handleInsights)
	mux.HandleFunc("/api/insights/weekly-narrative", s.handleWeeklyNarrative)

	// Static file serving — prefer embedded FS (prod), fall back to disk (dev).
	if s.embedFS != nil {
		mux.Handle("/", http.FileServer(http.FS(s.embedFS)))
	} else {
		webDir, err := findWebDist()
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: web/dist not found (%v) — UI will not load\n", err)
		} else {
			mux.Handle("/", http.FileServer(http.Dir(webDir)))
		}
	}

	handler := corsMiddleware(mux)

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("→ daimon dashboard at http://localhost%s\n", addr)
	return http.ListenAndServe(addr, handler)
}

// findWebDist locates the web/dist directory relative to the executable.
func findWebDist() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	// Resolve symlinks so we get the real binary location.
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(exe)
	candidate := filepath.Join(dir, "web", "dist")
	if info, err := os.Stat(candidate); err == nil && info.IsDir() {
		return candidate, nil
	}
	// Also try the current working directory (useful during development).
	wd, _ := os.Getwd()
	candidate2 := filepath.Join(wd, "web", "dist")
	if info, err := os.Stat(candidate2); err == nil && info.IsDir() {
		return candidate2, nil
	}
	return "", fmt.Errorf("web/dist not found near %s or %s", dir, wd)
}

// corsMiddleware adds permissive CORS headers for local development.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
