package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DB wraps sql.DB with breaklog-specific helpers.
type DB struct {
	*sql.DB
}

// Open opens (or creates) the breaklog SQLite database and runs migrations.
func Open() (*DB, error) {
	dir := DataDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	dbPath := filepath.Join(dir, "daimon.db")
	// WAL mode for safe concurrent access between daemon and CLI commands.
	sqlDB, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	// Serialize writes through a single connection.
	sqlDB.SetMaxOpenConns(1)

	db := &DB{sqlDB}
	if err := db.migrate(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

// DataDir returns the path to ~/.daimon/.
func DataDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".daimon")
}

// ShadowDir returns the path to ~/.breaklog/shadows/.
func ShadowDir() string {
	return filepath.Join(DataDir(), "shadows")
}

// PIDFile returns the path to the daemon PID file.
func PIDFile() string {
	return filepath.Join(DataDir(), "daemon.pid")
}

// ProjectsJSONPath returns the path to the projects registry JSON file.
func ProjectsJSONPath() string {
	return filepath.Join(DataDir(), "projects.json")
}

func (db *DB) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS projects (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			name        TEXT NOT NULL,
			path        TEXT NOT NULL UNIQUE,
			shadow_repo TEXT NOT NULL,
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id     INTEGER REFERENCES projects(id),
			started_at     DATETIME NOT NULL,
			ended_at       DATETIME,
			duration_sec   INTEGER,
			status         TEXT DEFAULT 'active',
			raw_diff       TEXT,
			summary        TEXT,
			files_changed  TEXT,
			snapshot_hash  TEXT,
			created_at     DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS config (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}
