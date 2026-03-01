# daimon

*The spirit that watches your code.*

Daimon is a silent background daemon that watches your coding sessions and tells you what you actually built — not just how long you coded. It tracks file changes, diffs what changed, and uses Claude AI to generate plain-English summaries of each session. All data stays local. No cloud, no account, no subscription.

---

## Install

**Mac / Linux**
```sh
curl -fsSL https://raw.githubusercontent.com/Kishanmp3/daimon/main/install.sh | sh
```

**Windows (PowerShell)**
```powershell
irm https://raw.githubusercontent.com/Kishanmp3/daimon/main/install.ps1 | iex
```

Then run first-time setup:
```sh
daimon summon
```

You need an Anthropic API key. Get one at [console.anthropic.com](https://console.anthropic.com). That's the only requirement.

---

## Quick start

```sh
# 1. Register a project
cd ~/projects/my-app
daimon haunt

# 2. Code. Daimon watches silently.

# 3. See what you built today
daimon recall

# 4. Open the web dashboard
daimon vision
```

---

## Commands

| Command | Description |
|---|---|
| `daimon summon` | First-time setup: save API key, register daemon as login startup |
| `daimon haunt [path]` | Register a project directory to watch (once per project) |
| `daimon recall` | Show today's sessions and AI summaries |
| `daimon manifest` | Weekly rollup of everything you built |
| `daimon manifest --week` | Same, scoped explicitly to the last 7 days |
| `daimon oracle "question"` | Ask AI anything about your coding history |
| `daimon vision` | Open the local web dashboard at localhost:4321 |
| `daimon status` | Show the current active session |
| `daimon end [path]` | Manually close a session and generate its summary |
| `daimon config set api-key sk-ant-...` | Update your Anthropic API key |

---

## How it works

1. `daimon haunt` registers a project and takes an initial file snapshot.
2. The daemon watches for file saves using filesystem events.
3. First save opens a session automatically.
4. After 30 minutes of inactivity the session closes.
5. Daimon diffs the current files against the opening snapshot.
6. The diff goes to Claude, which produces a plain-English summary.
7. Summary is stored locally and visible in `daimon recall` or `daimon vision`.

The diff system uses a shadow copy approach — no git required, works in any directory.

---

## Why not WakaTime?

WakaTime tells you you coded for 4 hours.

Daimon tells you you built a login form, fixed two auth bugs, and started on the password reset flow.

Time tracking answers *how long*. Daimon answers *what*. The summaries are written in plain English by Claude, not inferred from keystrokes. You can read them back a week later and actually know what you were doing.

---

## Storage

Everything lives in `~/.daimon/`:

```
~/.daimon/
├── daimon.db       — SQLite database (sessions, projects, config)
├── projects.json   — registered project paths
├── daemon.pid      — daemon process ID
└── shadows/        — file snapshots per project
```

Nothing leaves your machine except the diff sent to Claude's API when a session closes.

---

## Manual build

Requires Go 1.22+ and Node 20+.

```sh
# Build web UI first (needed for daimon vision)
cd web && npm ci && npm run build && cd ..

# Dev build (serves web/dist from disk)
go build -o daimon ./cmd/daimon

# Production build (web UI embedded in binary)
go build -tags prod -o daimon ./cmd/daimon
```

---

## License

MIT
