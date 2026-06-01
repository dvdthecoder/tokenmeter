package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"encoding/json"

	"github.com/dvdthecoder/tokenmeter/internal/config"
	"github.com/dvdthecoder/tokenmeter/internal/daemon"
	"github.com/dvdthecoder/tokenmeter/internal/insights"
	"github.com/dvdthecoder/tokenmeter/internal/mitm"
	"github.com/dvdthecoder/tokenmeter/internal/proxy"
	storage "github.com/dvdthecoder/tokenmeter/internal/storage/sqlite"
	"github.com/dvdthecoder/tokenmeter/plugins/backends"
	"github.com/dvdthecoder/tokenmeter/plugins/middleware"
	"github.com/dvdthecoder/tokenmeter/plugins/sinks"
	sqlitesink "github.com/dvdthecoder/tokenmeter/plugins/sinks/sqlite"

	// Blank imports register built-in plugins via init().
	_ "github.com/dvdthecoder/tokenmeter/plugins/backends/claudecode"
	_ "github.com/dvdthecoder/tokenmeter/plugins/backends/codex"
	_ "github.com/dvdthecoder/tokenmeter/plugins/backends/opencode"
	_ "github.com/dvdthecoder/tokenmeter/plugins/backends/vscode"
	_ "github.com/dvdthecoder/tokenmeter/plugins/middleware/redaction"
	_ "github.com/dvdthecoder/tokenmeter/plugins/providers/anthropic"
	_ "github.com/dvdthecoder/tokenmeter/plugins/providers/bedrock"
	_ "github.com/dvdthecoder/tokenmeter/plugins/providers/copilot"
	_ "github.com/dvdthecoder/tokenmeter/plugins/providers/gemini"
	_ "github.com/dvdthecoder/tokenmeter/plugins/providers/openai"
	_ "github.com/dvdthecoder/tokenmeter/plugins/sinks/otel"
	_ "github.com/dvdthecoder/tokenmeter/plugins/sinks/prometheus"
	_ "github.com/dvdthecoder/tokenmeter/plugins/sinks/sqlite"
	_ "github.com/dvdthecoder/tokenmeter/plugins/sinks/stdout"
)

// Version is set at build time via ldflags.
var Version = "dev"

func main() {
	root := &cobra.Command{
		Use:     "tokenmeter",
		Short:   "Token usage meter for LLM APIs",
		Version: Version,
	}

	root.AddCommand(
		cmdStart(),
		cmdDaemon(),
		cmdStop(),
		cmdStatus(),
		cmdInstall(),
		cmdUninstall(),
		cmdVerify(),
		cmdQuery(),
		cmdPurge(),
		cmdExport(),
		cmdInsights(),
		cmdDashboard(),
		cmdScaffold(),
		cmdCert(),
	)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func cmdStart() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start in foreground (dev mode)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, _ := cmd.Flags().GetString("config")
			var cfg *config.Config
			if cfgPath != "" {
				var err error
				cfg, err = config.Load(cfgPath)
				if err != nil {
					return fmt.Errorf("load config: %w", err)
				}
			} else {
				cfg = config.Default()
			}

			if cfg.Sinks == nil {
				cfg.Sinks = map[string]config.SinkConfig{}
			}
			// Dev defaults: stdout + sqlite always enabled in foreground mode.
			if _, ok := cfg.Sinks["stdout"]; !ok {
				cfg.Sinks["stdout"] = config.SinkConfig{
					Enabled: true,
					Options: map[string]any{"enabled": true},
				}
			}
			if _, ok := cfg.Sinks["sqlite"]; !ok {
				cfg.Sinks["sqlite"] = config.SinkConfig{
					Enabled: true,
					Options: map[string]any{},
				}
			}

			for name, sc := range cfg.Sinks {
				if !sc.Enabled {
					continue
				}
				sink, ok := sinks.Get(name)
				if !ok {
					slog.Warn("unknown sink in config", "sink", name)
					continue
				}
				if err := sink.Init(sc.Options); err != nil {
					return fmt.Errorf("init sink %s: %w", name, err)
				}
				slog.Info("sink enabled", "sink", name)
			}

			for _, mc := range cfg.Middleware {
				mw, ok := middleware.Get(mc.Name)
				if !ok {
					slog.Warn("unknown middleware in config", "middleware", mc.Name)
					continue
				}
				if err := mw.Init(mc.Options); err != nil {
					return fmt.Errorf("init middleware %s: %w", mc.Name, err)
				}
				slog.Info("middleware enabled", "middleware", mc.Name)
			}

			// Resolve the active SQLite path so dashboard/API handlers read
			// the same file the sqlite sink writes to, not always the default.
			activeSQLitePath := sqlitesink.DefaultDBPath()
			if sc, ok := cfg.Sinks["sqlite"]; ok {
				if p, _ := sc.Options["path"].(string); p != "" {
					activeSQLitePath = p
				}
			}

			if cfg.Privacy.DataMinimisation {
				slog.Warn("data_minimisation enabled — username, session, client, service_id stripped before sinks")
				if cfg.Privacy.HashUser {
					slog.Warn("data_minimisation + hash_user both set; hash_user has no effect in minimisation mode")
				}
			}

			p := proxy.New(cfg)
			mux := http.NewServeMux()
			mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				fmt.Fprintln(w, "ok")
			})
			mux.HandleFunc("/insights/latest", func(w http.ResponseWriter, r *http.Request) {
				db, err := storage.Open(activeSQLitePath)
				if err != nil {
					http.Error(w, "db unavailable", http.StatusServiceUnavailable)
					return
				}
				defer db.Close()
				ins, err := db.LatestInsight()
				if err != nil {
					http.Error(w, "query failed", http.StatusInternalServerError)
					return
				}
				if ins == nil {
					http.Error(w, "no insights yet — run: tokenmeter insights", http.StatusNotFound)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(ins)
			})
			mux.HandleFunc("/dashboard", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				fmt.Fprint(w, dashboardHTML)
			})
			mux.HandleFunc("/api/v1/events", func(w http.ResponseWriter, r *http.Request) {
				db, err := storage.Open(activeSQLitePath)
				if err != nil {
					http.Error(w, "db unavailable", http.StatusServiceUnavailable)
					return
				}
				defer db.Close()
				opts := parseQueryOpts(r)
				rows, err := db.Query(opts)
				if err != nil {
					http.Error(w, "query failed", http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Access-Control-Allow-Origin", "")
				if rows == nil {
					rows = []storage.Row{}
				}
				json.NewEncoder(w).Encode(rows)
			})
			mux.HandleFunc("/api/v1/stats", func(w http.ResponseWriter, r *http.Request) {
				db, err := storage.Open(activeSQLitePath)
				if err != nil {
					http.Error(w, "db unavailable", http.StatusServiceUnavailable)
					return
				}
				defer db.Close()
				opts := parseQueryOpts(r)
				stats, err := db.QueryStats(opts)
				if err != nil {
					http.Error(w, "query failed", http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(stats)
			})
			mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Claude Code CLI sends HEAD / as a pre-flight connectivity check.
				if r.Method == http.MethodHead && r.URL.Path == "/" {
					w.WriteHeader(http.StatusOK)
					return
				}
				p.ServeHTTP(w, r)
			}))

			// Wrap with MITM handler so CONNECT tunnels (Copilot, Bedrock) are intercepted.
			var handler http.Handler = mux
			ca, err := mitm.LoadOrCreate(dataDir())
			if err != nil {
				slog.Warn("mitm CA unavailable — CONNECT tunnels will be transparent", "err", err)
			} else {
				handler = &mitm.Handler{CA: ca, Next: mux}
				slog.Info("mitm CA ready", "cert", mitm.CertPath(dataDir()))
			}

			srv := &http.Server{
				Addr:    cfg.Proxy.Listen,
				Handler: handler,
				// Generous read timeout: SSE streams can be very long.
				// WriteTimeout deliberately unset — would cut streaming responses.
				ReadHeaderTimeout: 30 * time.Second,
			}

			go func() {
				slog.Info("tokenmeter listening", "addr", cfg.Proxy.Listen, "mode", cfg.Proxy.Mode)
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					slog.Error("server error", "err", err)
				}
			}()

			// Daily auto-generate insight if configured.
			if cfg.Insights.Enabled && cfg.Insights.AutoGenerate == "daily" {
				go func() {
					ticker := time.NewTicker(24 * time.Hour)
					defer ticker.Stop()
					for range ticker.C {
						db, err := storage.Open(activeSQLitePath)
						if err != nil {
							slog.Warn("insights: open db", "err", err)
							continue
						}
						_, err = insights.Run(context.Background(), db, cfg.Insights, nil)
						db.Close()
						if err != nil {
							slog.Warn("insights: auto-generate skipped", "err", err)
						} else {
							slog.Info("insights: auto-generated daily insight")
						}
					}
				}()
			}

			quit := make(chan os.Signal, 1)
			signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
			<-quit
			slog.Info("shutting down — draining in-flight requests")

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if err := srv.Shutdown(ctx); err != nil {
				slog.Warn("server shutdown timeout", "err", err)
			}
			for _, sink := range sinks.All() {
				if err := sink.Close(); err != nil {
					slog.Warn("sink close error", "sink", sink.Name(), "err", err)
				}
			}
			slog.Info("tokenmeter stopped")
			return nil
		},
	}
	cmd.Flags().String("config", "", "Path to config.yaml (optional)")
	return cmd
}

func cmdDaemon() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Start tokenmeter as a background daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			binary, err := os.Executable()
			if err != nil {
				return fmt.Errorf("resolve binary path: %w", err)
			}
			cfgPath, _ := cmd.Flags().GetString("config")
			pid, err := daemon.Start(binary, cfgPath)
			if err != nil {
				return err
			}
			fmt.Printf("tokenmeter started (pid %d)\n", pid)
			fmt.Printf("logs: %s\n", daemon.LogPath())
			return nil
		},
	}
	cmd.Flags().String("config", daemon.DefaultConfigPath(), "Path to config.yaml")
	return cmd
}

func cmdStop() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the running daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := daemon.Stop(); err != nil {
				return err
			}
			fmt.Println("tokenmeter stopped")
			return nil
		},
	}
}

func cmdStatus() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show daemon health and recent events",
		RunE: func(cmd *cobra.Command, args []string) error {
			pid, alive := daemon.ReadPID()
			if !alive {
				fmt.Println("status: stopped")
				if pid != 0 {
					fmt.Printf("stale PID file (pid %d) — daemon may have crashed\n", pid)
					fmt.Printf("log: %s\n", daemon.LogPath())
				}
				return nil
			}
			fmt.Printf("status:  running (pid %d)\n", pid)
			fmt.Printf("log:     %s\n", daemon.LogPath())

			db, err := storage.Open(sqlitesink.DefaultDBPath())
			if err != nil {
				return nil // DB not yet created is fine
			}
			defer db.Close()
			rows, err := db.Query(storage.QueryOpts{Limit: 5})
			if err != nil || len(rows) == 0 {
				return nil
			}
			fmt.Println("\nrecent events:")
			storage.WriteTable(os.Stdout, rows)
			return nil
		},
	}
}

func cmdInstall() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install daemon as system service and configure AI tools",
		RunE: func(cmd *cobra.Command, args []string) error {
			binary, err := os.Executable()
			if err != nil {
				return fmt.Errorf("resolve binary path: %w", err)
			}
			onlyBackend, _ := cmd.Flags().GetString("backend")

			// 1. Write default config (no-op if already exists).
			cfgPath, err := daemon.WriteDefaultConfig()
			if err != nil {
				return fmt.Errorf("write config: %w", err)
			}
			fmt.Printf("config:  %s\n", cfgPath)

			// 2. Install system service.
			if err := daemon.InstallService(binary, cfgPath); err != nil {
				return fmt.Errorf("install service: %w", err)
			}
			fmt.Println("service: installed and started")

			// 3. Patch shell profile with env vars.
			rcFile, err := daemon.PatchShell()
			if err != nil {
				fmt.Printf("warning: could not patch shell profile: %v\n", err)
			} else {
				fmt.Printf("shell:   patched %s\n", rcFile)
				fmt.Println("         → restart your shell or run: source", rcFile)
			}

			// 4. Configure detected AI tool backends.
			proxyAddr := config.Default().Proxy.Listen
			for _, b := range backends.All() {
				if onlyBackend != "" && b.Name() != onlyBackend {
					continue
				}
				if !b.Detect() {
					continue
				}
				if err := b.Install(proxyAddr); err != nil {
					fmt.Printf("warning: %s backend install: %v\n", b.Name(), err)
				} else {
					fmt.Printf("backend: %-12s configured\n", b.Name())
				}
			}

			fmt.Printf("logs:    %s\n", daemon.LogPath())
			fmt.Println("\ntokenmeter is running. Run 'tokenmeter verify' to confirm routing.")
			return nil
		},
	}
	cmd.Flags().String("backend", "", "Configure only a specific backend (claudecode|codex|opencode|vscode)")
	return cmd
}

func cmdVerify() *cobra.Command {
	return &cobra.Command{
		Use:   "verify",
		Short: "Verify proxy routing for installed AI tools",
		RunE: func(cmd *cobra.Command, args []string) error {
			proxyAddr := config.Default().Proxy.Listen

			// 1. Check proxy health.
			resp, err := http.Get("http://" + proxyAddr + "/health")
			if err != nil {
				fmt.Printf("proxy:        FAIL — not reachable at %s (%v)\n", proxyAddr, err)
			} else {
				resp.Body.Close()
				fmt.Printf("proxy:        OK   (%s)\n", proxyAddr)
			}

			// 2. Check each detected backend.
			found := false
			for _, b := range backends.All() {
				if !b.Detect() {
					continue
				}
				found = true
				if verr := b.Verify(proxyAddr); verr != nil {
					fmt.Printf("%-14s FAIL — %v\n", b.Name()+":", verr)
				} else {
					fmt.Printf("%-14s OK\n", b.Name()+":")
				}
			}
			if !found {
				fmt.Println("no AI tool backends detected on this machine")
			}
			return nil
		},
	}
}

func cmdUninstall() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Remove daemon service and revert shell configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := daemon.UninstallService(); err != nil {
				fmt.Printf("warning: remove service: %v\n", err)
			} else {
				fmt.Println("service: removed")
			}

			rcFile, err := daemon.UnpatchShell()
			if err != nil {
				fmt.Printf("warning: unpatch shell: %v\n", err)
			} else {
				fmt.Printf("shell:   unpatched %s\n", rcFile)
				fmt.Println("         → restart your shell for env vars to clear")
			}
			return nil
		},
	}
}

func cmdQuery() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query",
		Short: "Query local token usage data",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openDB(cmd)
			if err != nil {
				return err
			}
			defer db.Close()

			lastStr, _ := cmd.Flags().GetString("last")
			format, _ := cmd.Flags().GetString("format")
			model, _ := cmd.Flags().GetString("model")
			user, _ := cmd.Flags().GetString("user")
			limit, _ := cmd.Flags().GetInt("limit")

			opts := storage.QueryOpts{Model: model, User: user, Limit: limit}
			if lastStr != "" {
				d, err := parseDuration(lastStr)
				if err != nil {
					return fmt.Errorf("--last: %w", err)
				}
				opts.Since = time.Now().Add(-d)
			}

			rows, err := db.Query(opts)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}

			switch format {
			case "json":
				return storage.WriteJSON(os.Stdout, rows)
			case "csv":
				return storage.WriteCSV(os.Stdout, rows)
			default:
				storage.WriteTable(os.Stdout, rows)
				return nil
			}
		},
	}
	cmd.Flags().String("last", "24h", "Show events from the last duration (e.g. 1h, 7d)")
	cmd.Flags().String("format", "table", "Output format: table|json|csv")
	cmd.Flags().String("model", "", "Filter by model name")
	cmd.Flags().String("user", "", "Filter by username")
	cmd.Flags().Int("limit", 500, "Maximum number of rows (0 = unlimited)")
	cmd.Flags().String("db", "", "Path to SQLite database (default: ~/.local/share/tokenmeter/events.db)")
	return cmd
}

func cmdPurge() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "purge",
		Short: "GDPR-compliant event deletion",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openDB(cmd)
			if err != nil {
				return err
			}
			defer db.Close()

			beforeStr, _ := cmd.Flags().GetString("before")
			retentionDays, _ := cmd.Flags().GetInt("retention-days")
			user, _ := cmd.Flags().GetString("user")

			// Per-user purge (GDPR right-to-erasure).
			if user != "" {
				n, err := db.PurgeUser(user)
				if err != nil {
					return err
				}
				fmt.Fprintf(os.Stdout, "Deleted %d event(s) for user %q\n", n, user)
				return nil
			}

			var before time.Time
			switch {
			case beforeStr != "":
				before, err = time.Parse("2006-01-02", beforeStr)
				if err != nil {
					before, err = time.Parse(time.RFC3339, beforeStr)
				}
				if err != nil {
					return fmt.Errorf("--before: use YYYY-MM-DD or RFC3339")
				}
			case retentionDays > 0:
				before = time.Now().AddDate(0, 0, -retentionDays)
			default:
				return fmt.Errorf("specify --before <date>, --retention-days <n>, or --user <name>")
			}

			n, err := db.Purge(before)
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "Deleted %d event(s) before %s\n", n, before.UTC().Format("2006-01-02"))
			return nil
		},
	}
	cmd.Flags().String("before", "", "Delete events before this date (YYYY-MM-DD or RFC3339)")
	cmd.Flags().Int("retention-days", 0, "Delete events older than N days")
	cmd.Flags().String("user", "", "Delete all events for this user (GDPR right-to-erasure)")
	cmd.Flags().String("db", "", "Path to SQLite database")
	return cmd
}

func cmdExport() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export all token usage data",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openDB(cmd)
			if err != nil {
				return err
			}
			defer db.Close()

			format, _ := cmd.Flags().GetString("format")
			rows, err := db.Query(storage.QueryOpts{})
			if err != nil {
				return err
			}

			switch format {
			case "csv":
				return storage.WriteCSV(os.Stdout, rows)
			default:
				return storage.WriteJSON(os.Stdout, rows)
			}
		},
	}
	cmd.Flags().String("format", "json", "Output format: json|csv")
	cmd.Flags().String("db", "", "Path to SQLite database")
	return cmd
}

func cmdScaffold() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scaffold",
		Short: "Scaffold a new plugin",
	}
	cmd.AddCommand(
		&cobra.Command{Use: "provider", Short: "Scaffold a new provider plugin", RunE: func(cmd *cobra.Command, args []string) error { return nil }},
		&cobra.Command{Use: "sink", Short: "Scaffold a new sink plugin", RunE: func(cmd *cobra.Command, args []string) error { return nil }},
		&cobra.Command{Use: "backend", Short: "Scaffold a new backend adapter", RunE: func(cmd *cobra.Command, args []string) error { return nil }},
	)
	return cmd
}

// openDB opens the SQLite database using the --db flag or the default path.
func openDB(cmd *cobra.Command) (*storage.DB, error) {
	path, _ := cmd.Flags().GetString("db")
	if path == "" {
		path = sqlitesink.DefaultDBPath()
	}
	db, err := storage.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open database %s: %w", path, err)
	}
	return db, nil
}

// parseDuration extends time.ParseDuration to support "d" (days) suffix.
func parseDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("invalid duration %q: expected a positive integer before 'd'", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func cmdCert() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cert",
		Short: "Manage the local MITM CA certificate (required for Copilot interception)",
	}
	cmd.AddCommand(cmdCertInstall(), cmdCertPath())
	return cmd
}

func cmdCertInstall() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Generate CA cert and install it in the system trust store",
		RunE: func(cmd *cobra.Command, args []string) error {
			ca, err := mitm.LoadOrCreate(dataDir())
			if err != nil {
				return fmt.Errorf("generate CA: %w", err)
			}
			certPath := mitm.CertPath(dataDir())
			fmt.Fprintf(os.Stdout, "CA certificate: %s\n\n", certPath)

			// Platform-specific trust store install.
			switch {
			case fileExists("/usr/bin/security"): // macOS
				fmt.Fprintln(os.Stdout, "Installing to macOS system keychain (may prompt for password)...")
				return runCmd("security", "add-trusted-cert", "-d", "-r", "trustRoot",
					"-k", "/Library/Keychains/System.keychain", certPath)
			case fileExists("/usr/bin/update-ca-certificates"): // Debian/Ubuntu
				dest := "/usr/local/share/ca-certificates/tokenmeter.crt"
				if err := copyFile(certPath, dest); err != nil {
					return fmt.Errorf("copy cert: %w", err)
				}
				return runCmd("update-ca-certificates")
			case fileExists("/usr/bin/trust"): // Fedora/Arch
				return runCmd("trust", "anchor", "--store", certPath)
			default:
				fmt.Fprintf(os.Stdout, "Automatic install not supported on this platform.\n")
				fmt.Fprintf(os.Stdout, "Manually trust: %s\n", certPath)
				fmt.Fprintf(os.Stdout, "\nFor VS Code: add to settings.json:\n")
				fmt.Fprintf(os.Stdout, `  "http.proxy": "http://127.0.0.1:4191",`+"\n")
				fmt.Fprintf(os.Stdout, `  "http.proxyStrictSSL": false`+"\n")
			}
			_ = ca
			return nil
		},
	}
}

func cmdCertPath() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the path to the CA certificate",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(mitm.CertPath(dataDir()))
		},
	}
}

// dataDir returns the platform data directory for tokenmeter state files.
func dataDir() string {
	home, _ := os.UserHomeDir()
	return home + "/.local/share/tokenmeter"
}

func cmdInsights() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "insights",
		Short: "Generate AI insights from local usage data via Ollama",
		Long: `Queries the local SQLite database, builds an aggregated (no prompt/response content)
usage summary, sends it to a locally running Ollama SLM, stores the result, and
streams it to the terminal. Requires Ollama to be running (https://ollama.com).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			showOnly, _ := cmd.Flags().GetBool("show")
			modelOverride, _ := cmd.Flags().GetString("model")
			lastStr, _ := cmd.Flags().GetString("last")

			db, err := openDB(cmd)
			if err != nil {
				return err
			}
			defer db.Close()

			// --show: print the latest stored insight without generating.
			if showOnly {
				ins, err := db.LatestInsight()
				if err != nil {
					return err
				}
				if ins == nil {
					fmt.Fprintln(os.Stdout, "No insights stored yet. Run 'tokenmeter insights' to generate one.")
					return nil
				}
				fmt.Fprintf(os.Stdout, "Generated: %s  Model: %s  Window: %dd\n\n",
					ins.GeneratedAt.UTC().Format("2006-01-02 15:04 UTC"), ins.Model, ins.WindowDays)
				fmt.Fprintln(os.Stdout, ins.Content)
				return nil
			}

			cfg := config.Default()
			if modelOverride != "" {
				cfg.Insights.Model = modelOverride
			}
			if lastStr != "" {
				d, err := parseDuration(lastStr)
				if err != nil {
					return fmt.Errorf("--last: %w", err)
				}
				cfg.Insights.WindowDays = int(d.Hours() / 24)
				if cfg.Insights.WindowDays < 1 {
					cfg.Insights.WindowDays = 1
				}
			}

			fmt.Fprintf(os.Stdout, "Querying %d days of usage data → %s @ %s\n\n",
				cfg.Insights.WindowDays, cfg.Insights.Model, cfg.Insights.OllamaURL)

			ins, err := insights.Run(cmd.Context(), db, cfg.Insights, func(token string) {
				fmt.Print(token)
			})
			fmt.Println() // newline after streaming output
			if err != nil {
				return fmt.Errorf("insights: %w\n\nMake sure Ollama is running: https://ollama.com\nThen pull the model: ollama pull %s", err, cfg.Insights.Model)
			}
			fmt.Fprintf(os.Stdout, "\n[stored as %s]\n", ins.ID)
			return nil
		},
	}
	cmd.Flags().Bool("show", false, "Print the latest stored insight without generating a new one")
	cmd.Flags().String("model", "", "Ollama model to use (default: llama3.2:3b)")
	cmd.Flags().String("last", "", "Analyze events from the last duration, e.g. 7d, 30d (default: 7d)")
	cmd.Flags().String("db", "", "Path to SQLite database")
	return cmd
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// errNotYet returns a clear error for commands not yet implemented.
func errNotYet(cmd string) error {
	return fmt.Errorf("%q is not yet implemented (coming in a future release)", cmd)
}

// parseQueryOpts converts URL query params (?last=6h&limit=200) into QueryOpts.
func parseQueryOpts(r *http.Request) storage.QueryOpts {
	q := r.URL.Query()
	opts := storage.QueryOpts{Limit: 500}
	if last := q.Get("last"); last != "" {
		if d, err := parseDuration(last); err == nil {
			opts.Since = time.Now().Add(-d)
		}
	}
	if lim := q.Get("limit"); lim != "" {
		if n, err := strconv.Atoi(lim); err == nil && n > 0 {
			opts.Limit = n
		}
	}
	if m := q.Get("model"); m != "" {
		opts.Model = m
	}
	if u := q.Get("user"); u != "" {
		opts.User = u
	}
	return opts
}

func cmdDashboard() *cobra.Command {
	return &cobra.Command{
		Use:   "dashboard",
		Short: "Open the web dashboard (requires proxy to be running)",
		Run: func(cmd *cobra.Command, args []string) {
			addr := config.Default().Proxy.Listen
			url := "http://" + addr + "/dashboard"
			fmt.Printf("Dashboard: %s\n", url)
			// Try to open in browser on supported platforms.
			_ = exec.Command("open", url).Start()
		},
	}
}

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>tokenmeter</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;background:#0d1117;color:#c9d1d9;font-size:13px}
header{padding:14px 24px;border-bottom:1px solid #21262d;display:flex;align-items:center;gap:10px}
header h1{font-size:15px;font-weight:600;color:#f0f6fc;letter-spacing:-.3px}
.live{background:#1a7f37;color:#fff;padding:2px 8px;border-radius:10px;font-size:10px;font-weight:600}
#updated{margin-left:auto;font-size:11px;color:#6e7681}
.controls{padding:10px 24px;border-bottom:1px solid #21262d;display:flex;gap:6px;align-items:center}
.controls span{font-size:11px;color:#6e7681;margin-right:4px}
.btn{padding:3px 10px;border:1px solid #30363d;background:#161b22;color:#c9d1d9;border-radius:5px;cursor:pointer;font-size:12px;transition:background .1s}
.btn:hover{background:#21262d}
.btn.on{background:#1f6feb;border-color:#1f6feb;color:#fff}
.stats{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:1px;background:#21262d;border-bottom:1px solid #21262d}
.stat{background:#0d1117;padding:14px 20px}
.stat .lbl{font-size:10px;color:#6e7681;text-transform:uppercase;letter-spacing:.06em;margin-bottom:5px}
.stat .val{font-size:22px;font-weight:600;color:#e6edf3;font-variant-numeric:tabular-nums}
.stat .sub{font-size:10px;color:#6e7681;margin-top:3px}
.tbl-wrap{overflow:auto}
table{width:100%;border-collapse:collapse}
th{padding:7px 12px;text-align:left;font-size:10px;text-transform:uppercase;letter-spacing:.06em;color:#6e7681;border-bottom:1px solid #21262d;white-space:nowrap;background:#0d1117;position:sticky;top:0}
td{padding:7px 12px;border-bottom:1px solid #161b22;white-space:nowrap;font-variant-numeric:tabular-nums}
tr:hover td{background:#161b22}
.mono{font-family:'SF Mono',Consolas,monospace;font-size:11px}
.sess{color:#58a6ff;font-family:monospace;font-size:11px;cursor:default}
.green{color:#3fb950}
.chip{display:inline-block;padding:1px 6px;border-radius:4px;font-size:10px;background:#161b22;border:1px solid #21262d;font-family:monospace}
.empty{text-align:center;color:#6e7681;padding:48px 0}
</style>
</head>
<body>
<header>
  <h1>tokenmeter</h1>
  <span class="live">&#9679; live</span>
  <span id="updated"></span>
</header>
<div class="controls">
  <span>LAST</span>
  <button class="btn" onclick="pick(this,'1h')">1h</button>
  <button class="btn on" onclick="pick(this,'6h')">6h</button>
  <button class="btn" onclick="pick(this,'24h')">24h</button>
  <button class="btn" onclick="pick(this,'7d')">7d</button>
  <button class="btn" onclick="pick(this,'30d')">30d</button>
</div>
<div class="stats">
  <div class="stat"><div class="lbl">Requests</div><div class="val" id="s-req">—</div><div class="sub" id="s-sess"></div></div>
  <div class="stat"><div class="lbl">Input tokens</div><div class="val" id="s-in">—</div></div>
  <div class="stat"><div class="lbl">Output tokens</div><div class="val" id="s-out">—</div></div>
  <div class="stat"><div class="lbl">Cached tokens</div><div class="val" id="s-cache">—</div></div>
  <div class="stat"><div class="lbl">Total cost</div><div class="val green" id="s-cost">—</div></div>
  <div class="stat"><div class="lbl">Avg latency</div><div class="val" id="s-lat">—</div></div>
</div>
<div class="tbl-wrap">
<table>
  <thead><tr>
    <th>Time</th><th>Session</th><th>Client</th><th>Model</th>
    <th>In</th><th>Out</th><th>Cached</th><th>Cost</th><th>Latency</th><th>User</th>
  </tr></thead>
  <tbody id="tbody"></tbody>
</table>
</div>
<script>
let win='6h';
function pick(btn,w){
  document.querySelectorAll('.btn').forEach(b=>b.classList.remove('on'));
  btn.classList.add('on');
  win=w;refresh();
}
function n(v){
  if(v>=1e9)return(v/1e9).toFixed(1)+'B';
  if(v>=1e6)return(v/1e6).toFixed(1)+'M';
  if(v>=1e3)return(v/1e3).toFixed(1)+'K';
  return''+v;
}
function cost(c){return c===0?'$0.000000':'$'+c.toFixed(6)}
function lat(ms){return ms>=1000?(ms/1000).toFixed(1)+'s':ms+'ms'}
function sess(id){
  if(!id)return'<span style="color:#6e7681">—</span>';
  const s=id.length>14?id.slice(0,14)+'…':id;
  return'<span class="sess" title="'+id+'">'+s+'</span>';
}
async function refresh(){
  try{
    const[sr,er]=await Promise.all([
      fetch('/api/v1/stats?last='+win),
      fetch('/api/v1/events?last='+win+'&limit=300')
    ]);
    const s=await sr.json(),ev=await er.json();
    document.getElementById('s-req').textContent=n(s.requests||0);
    document.getElementById('s-sess').textContent=s.sessions>0?s.sessions+' session'+(s.sessions!==1?'s':''):'';
    document.getElementById('s-in').textContent=n(s.tokens_input||0);
    document.getElementById('s-out').textContent=n(s.tokens_output||0);
    document.getElementById('s-cache').textContent=n(s.tokens_cached||0);
    document.getElementById('s-cost').textContent=cost(s.cost_usd||0);
    document.getElementById('s-lat').textContent=s.requests>0?lat(Math.round(s.latency_ms_avg||0)):'—';
    document.getElementById('updated').textContent='Updated '+new Date().toLocaleTimeString();
    const tb=document.getElementById('tbody');
    if(!ev||ev.length===0){
      tb.innerHTML='<tr><td colspan="10" class="empty">No events in window</td></tr>';
      return;
    }
    tb.innerHTML=ev.map(e=>` + "`" + `
      <tr>
        <td class="mono">${new Date(e.timestamp).toLocaleTimeString()}</td>
        <td>${sess(e.session_id)}</td>
        <td><span class="chip">${e.client_name||'—'}</span></td>
        <td class="mono">${e.model||'—'}</td>
        <td>${n(e.tokens_input)}</td>
        <td>${n(e.tokens_output)}</td>
        <td>${n(e.tokens_cached)}</td>
        <td class="green">${cost(e.cost_usd)}</td>
        <td>${lat(e.latency_ms)}</td>
        <td>${e.username||'—'}</td>
      </tr>` + "`" + `).join('');
  }catch(e){console.error(e)}
}
refresh();
setInterval(refresh,10000);
</script>
</body>
</html>`
