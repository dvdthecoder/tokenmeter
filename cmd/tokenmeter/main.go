package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/dvdthecoder/tokenmeter/internal/config"
	"github.com/dvdthecoder/tokenmeter/internal/daemon"
	"github.com/dvdthecoder/tokenmeter/internal/proxy"
	storage "github.com/dvdthecoder/tokenmeter/internal/storage/sqlite"
	"github.com/dvdthecoder/tokenmeter/plugins/sinks"
	sqlitesink "github.com/dvdthecoder/tokenmeter/plugins/sinks/sqlite"

	// Blank imports register built-in plugins via init().
	_ "github.com/dvdthecoder/tokenmeter/plugins/providers/anthropic"
	_ "github.com/dvdthecoder/tokenmeter/plugins/providers/openai"
	_ "github.com/dvdthecoder/tokenmeter/plugins/sinks/sqlite"
	_ "github.com/dvdthecoder/tokenmeter/plugins/sinks/stdout"
	// Future iterations:
	// _ "github.com/dvdthecoder/tokenmeter/plugins/sinks/otel"
	// _ "github.com/dvdthecoder/tokenmeter/plugins/backends/claudecode"
	// _ "github.com/dvdthecoder/tokenmeter/plugins/backends/codex"
	// _ "github.com/dvdthecoder/tokenmeter/plugins/backends/opencode"
	// _ "github.com/dvdthecoder/tokenmeter/plugins/backends/vscode"
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
		cmdQuery(),
		cmdPurge(),
		cmdExport(),
		cmdScaffold(),
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

			p := proxy.New(cfg)
			srv := &http.Server{
				Addr:    cfg.Proxy.Listen,
				Handler: p,
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

			fmt.Printf("logs:    %s\n", daemon.LogPath())
			fmt.Println("\ntokenmeter is running. Use 'tokenmeter status' to verify.")
			return nil
		},
	}
	cmd.Flags().String("backend", "", "Configure only a specific backend (claudecode|codex|opencode|vscode) — coming in Iteration 4")
	return cmd
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
				return fmt.Errorf("specify --before <date> or --retention-days <n>")
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

// errNotYet returns a clear error for commands not yet implemented.
func errNotYet(cmd string) error {
	return fmt.Errorf("%q is not yet implemented (coming in a future release)", cmd)
}
