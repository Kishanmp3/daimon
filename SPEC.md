# breaklog — Product Specification

## What it is

breaklog is a local-first CLI tool for solo developers that automatically tracks coding sessions and generates plain-English summaries of what was built — without requiring any git commits, manual timers, or extra steps from the developer.

The developer just codes. breaklog runs silently in the background, detects when a session starts and ends, diffs what changed, and produces a readable summary powered by an LLM.

**One sentence:** breaklog is like WakaTime, but instead of telling you how long you coded, it tells you what you actually built.

---

## Core Principles

- **Zero friction.** The developer should never be interrupted or required to do extra work.
- **Local first.** All data lives in a single SQLite file on the developer's machine. No account, no cloud sync, no subscriptions.
- **Honest summaries.** Summaries are generated from real code changes, not just time spent.
- **Fast and lightweight.** The daemon should be invisible — no noticeable CPU or memory usage.

---

## How it Works

### Session Detection
A background daemon (`breaklog daemon`) watches for file system activity in registered project directories. When file saves are detected, a session begins. When there's been no file activity for **30 minutes**, the session is automatically closed.

### Shadow Diffing (the key insight)
breaklog maintains a **shadow git repository** (hidden, separate from the user's actual git history) for each watched project. At session start, it takes a snapshot. At session end, it generates a diff of everything that changed — regardless of whether the developer committed anything. This diff is compact (only changed lines) and gets sent to the LLM for summarization.

If the project already has git and the developer did commit, breaklog uses the real git diff instead — it's richer and more accurate.

### AI Summarization
The diff (shadow or real) is sent to the Anthropic API (Claude). The prompt instructs it to produce a short, plain-English summary of:
- What was built or changed
- What problem it appears to solve
- What tech/files were involved

The summary is stored in SQLite and displayed in the terminal at session end.

---

## Tech Stack

| Layer | Choice | Reason |
|---|---|---|
| Language | **Go** | Single binary, fast, easy to install (`go install`), good for daemons and CLI |
| Database | **SQLite** (via `modernc.org/sqlite`) | Local, zero config, single file |
| CLI framework | **Cobra** | Standard Go CLI library |
| TUI | **Bubble Tea + Lip Gloss** | Same stack as micasa, polished terminal UI |
| AI | **Anthropic API** (Claude claude-sonnet-4-20250514) | Best summarization quality |
| Web UI | **React + Vite** (served locally) | Built later — not MVP scope |
| File watching | **fsnotify** | Cross-platform file system events in Go |
| Shadow git | **go-git** | Pure Go git implementation, no git binary required |

---

## Project Structure

```
breaklog/
├── cmd/
│   └── breaklog/
│       └── main.go          # Entry point
├── internal/
│   ├── daemon/              # Background watcher
│   │   ├── daemon.go        # Main daemon loop
│   │   └── watcher.go       # fsnotify file watcher
│   ├── session/             # Session lifecycle
│   │   ├── session.go       # Start, end, auto-close logic
│   │   └── snapshot.go      # Shadow git snapshot/diff
│   ├── db/                  # SQLite layer
│   │   ├── db.go            # Connection, migrations
│   │   └── queries.go       # CRUD operations
│   ├── ai/                  # LLM integration
│   │   └── summarize.go     # Anthropic API call + prompt
│   └── display/             # Terminal output formatting
│       └── display.go       # Lip Gloss styled output
├── go.mod
├── go.sum
└── README.md
```

---

## CLI Commands

### `breaklog daemon`
Starts the background daemon. Should be added to shell startup (`.zshrc` / `.bashrc`). Watches all registered project directories for file activity.

```
$ breaklog daemon
→ breaklog daemon running. Watching 3 projects.
```

### `breaklog watch [path]`
Registers a directory as a project to watch. If no path given, uses current directory.

```
$ breaklog watch
→ Now watching: /Users/kee/projects/my-app

$ breaklog watch ~/projects/other-app
→ Now watching: /Users/kee/projects/other-app
```

### `breaklog today`
Shows all sessions from today with summaries.

```
$ breaklog today

Thursday, Feb 26
────────────────────────────────────────
09:14  my-app       1h 52m
       Built JWT login endpoint, password hashing utility,
       wired login form to backend.

14:00  my-app       0h 34m
       Debugged null check in token validation middleware.

Total: 2h 26m
```

### `breaklog summary [--week] [--month]`
Shows a rolled-up summary across sessions.

```
$ breaklog summary --week

Week of Feb 24
────────────────────────────────────────
You shipped:
  - Full auth system (login, logout, JWT tokens)
  - Dashboard layout with responsive sidebar
  - Started user settings page (incomplete)

8 sessions · 14h 20m · my-app
```

### `breaklog ask "[question]"`
Ask the LLM questions about your session history. It uses stored summaries + diffs as context.

```
$ breaklog ask "what was I working on last Tuesday?"
→ Last Tuesday you had 2 sessions on my-app. You spent most
  of the time on the database schema — added users and sessions
  tables. You also had a short session fixing a Postgres
  connection issue.
```

### `breaklog status`
Shows current session status.

```
$ breaklog status
→ Active session on my-app (started 43min ago)
   Files changed so far: auth/routes.go, components/Login.jsx
```

### `breaklog sessions [--limit N]`
Lists recent sessions with one-line summaries.

```
$ breaklog sessions --limit 5
```

### `breaklog ui`
*(Post-MVP)* Starts a local web server and opens the dashboard at `localhost:4321`.

---

## Data Model (SQLite)

### `projects`
```sql
CREATE TABLE projects (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  name        TEXT NOT NULL,
  path        TEXT NOT NULL UNIQUE,
  shadow_repo TEXT NOT NULL,  -- path to hidden shadow git repo
  created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### `sessions`
```sql
CREATE TABLE sessions (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  project_id   INTEGER REFERENCES projects(id),
  started_at   DATETIME NOT NULL,
  ended_at     DATETIME,
  duration_sec INTEGER,
  status       TEXT DEFAULT 'active',  -- active | closed
  raw_diff     TEXT,                   -- the actual diff sent to AI
  summary      TEXT,                   -- AI-generated summary
  files_changed TEXT,                  -- JSON array of file paths
  created_at   DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### `config`
```sql
CREATE TABLE config (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
-- stores: anthropic_api_key, idle_timeout_minutes, etc.
```

---

## AI Summarization

### Setup
```
ANTHROPIC_API_KEY=your_key_here
```

Or set via:
```
$ breaklog config set api-key sk-ant-...
```

### Prompt sent to Claude

```
You are summarizing a developer's coding session for their personal log.

Here is the git diff of changes made during this session:
<diff>
{diff}
</diff>

Write a 2-4 sentence plain English summary of:
1. What was built or changed
2. What the code appears to do
3. What files or tech stack was involved

Be specific and concrete. Don't use filler phrases like "the developer worked on..."
Just describe what was built as if writing a commit message narrative.
Keep it under 60 words.
```

---

## Daemon Behavior

- Runs as a background process, started via `breaklog daemon`
- Polls registered project directories using `fsnotify`
- Ignores: `node_modules/`, `.git/`, `dist/`, `build/`, `__pycache__/`, binary files, files > 1MB
- Session starts on first file save detected
- Session auto-closes after **30 minutes of no file activity**
- On auto-close: generates diff, calls AI, stores summary, prints to terminal if a terminal is attached
- On system restart: daemon needs to be re-started (shell startup handles this)
- PID file stored at `~/.breaklog/daemon.pid`

---

## Config & Storage

All data stored in `~/.breaklog/`:
```
~/.breaklog/
├── breaklog.db          # SQLite database
├── daemon.pid           # Daemon process ID
├── config.json          # User config (api key, settings)
└── shadows/             # Shadow git repos per project
    ├── my-app/
    └── other-app/
```

---

## Installation

```sh
go install github.com/YOUR_USERNAME/breaklog/cmd/breaklog@latest
```

Add daemon to shell startup:
```sh
# in .zshrc or .bashrc
breaklog daemon &
```

---

## MVP Scope (build this first)

- [ ] `breaklog daemon` — file watcher, session detection, auto-close
- [ ] `breaklog watch` — register project directories  
- [ ] Shadow git diffing via go-git
- [ ] AI summarization via Anthropic API
- [ ] SQLite storage
- [ ] `breaklog today` — display today's sessions
- [ ] `breaklog summary --week` — weekly rollup
- [ ] `breaklog status` — current session info
- [ ] Terminal output with Lip Gloss styling

## Post-MVP

- [ ] `breaklog ask` — LLM chat over history
- [ ] `breaklog ui` — local React dashboard at localhost:4321
- [ ] `breaklog sessions` — full history browser
- [ ] Tech stack detection (infer from file extensions / package.json)
- [ ] Homebrew tap for easy installation

---

## What breaklog is NOT

- Not a time tracker (WakaTime, Toggl)
- Not a task manager (Linear, Notion)
- Not an AI coding assistant (Cursor, Claude Code)
- Not a git client

It is purely a **session journal** — a record of what you built, generated automatically.
