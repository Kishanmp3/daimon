package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	root "github.com/Kishanmp3/breaklog"
	"github.com/Kishanmp3/breaklog/internal/ai"
	"github.com/Kishanmp3/breaklog/internal/daemon"
	"github.com/Kishanmp3/breaklog/internal/db"
	"github.com/Kishanmp3/breaklog/internal/display"
	"github.com/Kishanmp3/breaklog/internal/server"
	sess "github.com/Kishanmp3/breaklog/internal/session"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "daimon",
	Short: "Automatic coding session logger — tracks what you built, not just how long.",
	Long: `daimon watches your projects silently in the background, detects coding
sessions, diffs what changed, and produces plain-English summaries powered by Claude.

Run 'daimon summon' to get started.`,
}

func init() {
	rootCmd.AddCommand(
		summonCmd,
		daemonCmd,
		hauntCmd,
		endCmd,
		recallCmd,
		manifestCmd,
		statusCmd,
		oracleCmd,
		visionCmd,
		configCmd,
	)
	manifestCmd.Flags().Bool("week", false, "Show a weekly rolled-up summary")
	manifestCmd.Flags().Bool("month", false, "Show a monthly rolled-up summary")
}

// ──────────────────────────────────────────────────────────────────
// summon — first-time setup
// ──────────────────────────────────────────────────────────────────

var summonCmd = &cobra.Command{
	Use:   "summon",
	Short: "First-time setup: save API key and register daimon to start on login",
	Long:  `Walks through first-time configuration: saves your Anthropic API key and registers the daimon daemon as a startup task so it runs automatically on every login.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		hr := strings.Repeat("─", 46)
		fmt.Println(hr)
		fmt.Println("  daimon — setup")
		fmt.Println(hr)
		fmt.Println()

		// ── Step 1: API key ──────────────────────────────────────
		fmt.Print("  Step 1/2 — Enter your Anthropic API key\n")
		fmt.Print("  (sk-ant-...): ")

		reader := bufio.NewReader(os.Stdin)
		apiKey, _ := reader.ReadString('\n')
		apiKey = strings.TrimSpace(apiKey)

		database, err := db.Open()
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer database.Close()

		if apiKey == "" {
			fmt.Println("  ⚠  No key entered. Skipping.")
			fmt.Println("     Set it later with: daimon config set api-key sk-ant-...")
		} else {
			if err := database.SetConfig("anthropic_api_key", apiKey); err != nil {
				return fmt.Errorf("save API key: %w", err)
			}
			fmt.Println("  ✓  API key saved.")
		}

		fmt.Println()

		// ── Step 2: Register auto-start ──────────────────────────
		fmt.Println("  Step 2/2 — Registering daimon as a startup task")

		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("find executable: %w", err)
		}
		exe, _ = filepath.EvalSymlinks(exe)

		if err := registerAutoStart(exe); err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠  Auto-start registration failed: %v\n", err)
			fmt.Println("     Start the daemon manually with: daimon daemon")
		} else {
			fmt.Println("  ✓  daimon will start automatically on login.")
		}

		fmt.Println()
		fmt.Println(hr)
		fmt.Println("  Daimon is summoned. Run daimon haunt in any project to begin tracking.")
		fmt.Println(hr)
		return nil
	},
}

// ──────────────────────────────────────────────────────────────────
// daemon
// ──────────────────────────────────────────────────────────────────

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Start the background file watcher",
	Long: `Starts the daimon background process. It watches all registered project
directories for file changes and automatically opens and closes coding sessions.

Run 'daimon summon' to register daimon as a startup task so it starts on login.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open()
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer database.Close()

		d := daemon.New(database)
		return d.Run()
	},
}

// ──────────────────────────────────────────────────────────────────
// haunt
// ──────────────────────────────────────────────────────────────────

var hauntCmd = &cobra.Command{
	Use:   "haunt [path]",
	Short: "Register a directory as a project to watch",
	Long:  `Registers a directory so the daemon will track coding sessions in it. Defaults to the current directory. Starts the daemon in the background if it is not already running.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var target string
		if len(args) == 1 {
			abs, err := filepath.Abs(args[0])
			if err != nil {
				return fmt.Errorf("resolve path: %w", err)
			}
			target = abs
		} else {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			target = wd
		}

		if info, err := os.Stat(target); err != nil || !info.IsDir() {
			return fmt.Errorf("%s is not a directory", target)
		}

		database, err := db.Open()
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer database.Close()

		name := db.ProjectNameFromPath(target)
		shadowDir := shadowPath(target)
		if err := os.MkdirAll(shadowDir, 0o755); err != nil {
			return fmt.Errorf("create shadow dir: %w", err)
		}

		fmt.Printf("Initialising shadow repo for %s …\n", name)
		if err := sess.InitShadow(target, shadowDir); err != nil {
			return fmt.Errorf("init shadow repo: %w", err)
		}
		if _, err := sess.TakeSnapshot(target, shadowDir); err != nil {
			fmt.Fprintf(os.Stderr, "warning: initial snapshot failed: %v\n", err)
		}

		if _, err := database.UpsertProject(target, name, shadowDir); err != nil {
			return fmt.Errorf("register project: %w", err)
		}

		// Persist the updated project list to projects.json.
		allProjects, _ := database.GetAllProjects()
		if err := saveProjectsJSON(allProjects); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not write projects.json: %v\n", err)
		}

		// Auto-start daemon in the background if it is not already running.
		if !isDaemonRunning() {
			if err := startDaemonBackground(); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not start daemon: %v\n  Run 'daimon daemon' manually.\n", err)
			}
		}

		fmt.Printf("→ Daimon is haunting %s. It will follow your work silently.\n", name)
		return nil
	},
}

// ──────────────────────────────────────────────────────────────────
// end
// ──────────────────────────────────────────────────────────────────

var endCmd = &cobra.Command{
	Use:   "end [path]",
	Short: "Manually close the active session for a project",
	Long: `Closes the current active coding session for the project in the current
directory (or the given path), generates a diff, calls the Anthropic API to
summarise the work, saves the result to the database, and prints the summary.

This is identical to what happens automatically after 30 minutes of inactivity.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var target string
		if len(args) == 1 {
			abs, err := filepath.Abs(args[0])
			if err != nil {
				return fmt.Errorf("resolve path: %w", err)
			}
			target = abs
		} else {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			target = wd
		}

		database, err := db.Open()
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer database.Close()

		project, err := database.GetProjectByPath(target)
		if err != nil {
			return fmt.Errorf("look up project: %w", err)
		}
		if project == nil {
			return fmt.Errorf("%s is not a registered project — run: daimon haunt", target)
		}

		session, err := database.GetActiveSessionForProject(project.ID)
		if err != nil {
			return fmt.Errorf("look up session: %w", err)
		}
		if session == nil {
			return fmt.Errorf("no active session for %s", project.Name)
		}

		apiKeyStored, _ := database.GetConfig("anthropic_api_key")
		apiKey := ai.GetAPIKey(apiKeyStored)

		fmt.Printf("Closing session for %s…\n", project.Name)
		if err := sess.Close(database, session, project, apiKey); err != nil {
			return fmt.Errorf("close session: %w", err)
		}

		closed, err := database.GetSessionByID(session.ID)
		if err != nil || closed == nil {
			return fmt.Errorf("reload session: %w", err)
		}

		display.PrintSessionClosed(closed)
		return nil
	},
}

// ──────────────────────────────────────────────────────────────────
// recall
// ──────────────────────────────────────────────────────────────────

var recallCmd = &cobra.Command{
	Use:   "recall",
	Short: "Show today's coding sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open()
		if err != nil {
			return err
		}
		defer database.Close()

		sessions, err := database.GetSessionsForToday()
		if err != nil {
			return err
		}
		display.ShowToday(sessions)
		return nil
	},
}

// ──────────────────────────────────────────────────────────────────
// manifest
// ──────────────────────────────────────────────────────────────────

var manifestCmd = &cobra.Command{
	Use:   "manifest",
	Short: "Show a rolled-up summary of sessions",
	Long:  `Aggregates recent sessions and asks Claude to produce a high-level summary of what you shipped.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		week, _ := cmd.Flags().GetBool("week")

		database, err := db.Open()
		if err != nil {
			return err
		}
		defer database.Close()

		if week {
			sessions, err := database.GetSessionsForWeek()
			if err != nil {
				return err
			}

			var summaries []string
			for _, s := range sessions {
				if s.Summary != "" && !strings.HasPrefix(s.Summary, "[") {
					summaries = append(summaries, s.Summary)
				}
			}

			var rollup string
			if len(summaries) > 0 {
				apiKeyStored, _ := database.GetConfig("anthropic_api_key")
				apiKey := ai.GetAPIKey(apiKeyStored)
				if apiKey != "" {
					rollup, err = ai.SummarizeWeek(summaries, apiKey)
					if err != nil {
						fmt.Fprintf(os.Stderr, "warning: AI rollup failed: %v\n", err)
					}
				}
			}

			display.ShowWeeklySummary(sessions, rollup)
			return nil
		}

		sessions, err := database.GetSessionsForToday()
		if err != nil {
			return err
		}
		display.ShowToday(sessions)
		return nil
	},
}

// ──────────────────────────────────────────────────────────────────
// status
// ──────────────────────────────────────────────────────────────────

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current session status",
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open()
		if err != nil {
			return err
		}
		defer database.Close()

		sessions, err := database.GetAllActiveSessions()
		if err != nil {
			return err
		}
		display.ShowStatus(sessions)
		return nil
	},
}

// ──────────────────────────────────────────────────────────────────
// oracle
// ──────────────────────────────────────────────────────────────────

var oracleCmd = &cobra.Command{
	Use:   `oracle "[question]"`,
	Short: "Ask daimon a question about your session history",
	Long:  `Queries your session history using natural language. Powered by Claude. (Coming soon — not yet implemented.)`,
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("→ oracle is not yet implemented.")
		fmt.Println("  Coming soon: ask questions about what you've built.")
		return nil
	},
}

// ──────────────────────────────────────────────────────────────────
// vision
// ──────────────────────────────────────────────────────────────────

var visionCmd = &cobra.Command{
	Use:   "vision",
	Short: "Open the web dashboard at localhost:4321",
	Long:  `Starts a local HTTP server and opens the daimon dashboard in your browser.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open()
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}

		embedFS, _ := root.WebDistFS()
		srv := server.New(database, embedFS)
		go openBrowser("http://localhost:4321")
		fmt.Println("→ daimon dashboard at http://localhost:4321")
		return srv.Run(4321)
	},
}

func openBrowser(url string) {
	switch runtime.GOOS {
	case "windows":
		_ = exec.Command("cmd", "/c", "start", url).Start()
	case "darwin":
		_ = exec.Command("open", url).Start()
	default:
		_ = exec.Command("xdg-open", url).Start()
	}
}

// ──────────────────────────────────────────────────────────────────
// config
// ──────────────────────────────────────────────────────────────────

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage daimon configuration",
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a config value",
	Example: `  daimon config set api-key sk-ant-...
  daimon config set idle-timeout 45`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key   := normaliseConfigKey(args[0])
		value := args[1]

		database, err := db.Open()
		if err != nil {
			return err
		}
		defer database.Close()

		if err := database.SetConfig(key, value); err != nil {
			return err
		}
		fmt.Printf("→ %s set.\n", args[0])
		return nil
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a config value",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := normaliseConfigKey(args[0])

		database, err := db.Open()
		if err != nil {
			return err
		}
		defer database.Close()

		value, err := database.GetConfig(key)
		if err != nil {
			return err
		}
		if value == "" {
			fmt.Println("(not set)")
		} else {
			if strings.Contains(key, "key") && len(value) > 8 {
				value = value[:8] + "…"
			}
			fmt.Println(value)
		}
		return nil
	},
}

func init() {
	configCmd.AddCommand(configSetCmd, configGetCmd)
}

// ──────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────

func normaliseConfigKey(key string) string {
	switch strings.ToLower(key) {
	case "api-key", "apikey", "api_key":
		return "anthropic_api_key"
	case "idle-timeout", "idle_timeout":
		return "idle_timeout_minutes"
	}
	return key
}

func shadowPath(projectPath string) string {
	name := db.ProjectNameFromPath(projectPath)
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, name)
	return filepath.Join(db.ShadowDir(), name)
}

// isDaemonRunning checks whether a daimon daemon process is currently alive
// by reading the PID file and probing the process.
func isDaemonRunning() bool {
	data, err := os.ReadFile(db.PIDFile())
	if err != nil {
		return false
	}
	pidStr := strings.TrimSpace(string(data))
	if pidStr == "" {
		return false
	}
	switch runtime.GOOS {
	case "windows":
		out, err := exec.Command("tasklist", "/fi", "PID eq "+pidStr, "/fo", "csv", "/nh").Output()
		if err != nil {
			return false
		}
		return strings.Contains(string(out), pidStr)
	default:
		return exec.Command("kill", "-0", pidStr).Run() == nil
	}
}

// startDaemonBackground launches `daimon daemon` as a detached background process.
func startDaemonBackground() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, _ = filepath.EvalSymlinks(exe)

	cmd := exec.Command(exe, "daemon")
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	return cmd.Start()
}

// projectJSONEntry is the shape of a single entry in projects.json.
type projectJSONEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// saveProjectsJSON writes the current project list to ~/.daimon/projects.json.
func saveProjectsJSON(projects []*db.Project) error {
	entries := make([]projectJSONEntry, len(projects))
	for i, p := range projects {
		entries[i] = projectJSONEntry{Name: p.Name, Path: p.Path}
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(db.ProjectsJSONPath(), data, 0o644)
}
