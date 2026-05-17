package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/helix-dev-tools/internal/config"
	"github.com/nfsarch33/helix-dev-tools/internal/ctxmode"
	"github.com/nfsarch33/helix-dev-tools/internal/logger"
	"github.com/nfsarch33/helix-dev-tools/internal/metrics"
)

var sessionIndexConfigPath string

var sessionIndexCmd = &cobra.Command{
	Use:   "session-index",
	Short: "Pre-index key files via context-mode for BM25 search during session",
	Long: `Index frequently-referenced files (roadmap, daily-startup, SOPs) into
the context-mode BM25 knowledge base so they are searchable via ctx_search
without consuming context window tokens.

Uses a config file at ~/.cursor/session-index.json if present, otherwise
falls back to built-in defaults (daily-startup-prompt, active roadmap, sprint SOP).

This is a manual subcommand. Cursor does not support a sessionStart hook event,
so this must be invoked explicitly or via an external trigger.`,
	RunE: runSessionIndex,
}

func init() {
	sessionIndexCmd.Flags().StringVarP(&sessionIndexConfigPath, "config", "c", "",
		"Path to session-index config JSON (default: ~/.cursor/session-index.json)")
}

func runSessionIndex(_ *cobra.Command, _ []string) error {
	started := time.Now()
	paths := config.DefaultPaths()
	log := logger.New(paths.LogFile("session-index"))

	var targets []ctxmode.IndexTarget
	var timeoutSec int

	cfgPath := sessionIndexConfigPath
	if cfgPath == "" {
		cfgPath = ctxmode.DefaultConfigPath(paths.Home)
	}

	cfg, err := ctxmode.LoadConfig(cfgPath)
	if err != nil {
		log.Log(fmt.Sprintf("config not loaded (%s), using defaults", err))
		targets = ctxmode.DefaultTargets(paths.Home)
		timeoutSec = 30
	} else {
		targets = cfg.Targets
		timeoutSec = cfg.TimeoutSec
		if timeoutSec <= 0 {
			timeoutSec = 30
		}
	}

	fmt.Fprintf(os.Stderr, "session-index: indexing %d targets (timeout=%ds)\n", len(targets), timeoutSec)

	results := ctxmode.RunBatchIndex(targets, timeoutSec)

	var indexed, skipped, errored int
	for _, r := range results {
		switch r.Status {
		case "indexed":
			indexed++
			fmt.Fprintf(os.Stderr, "  ✓ %s\n", r.Source)
		case "skipped":
			skipped++
			fmt.Fprintf(os.Stderr, "  ○ %s (%s)\n", r.Source, r.Error)
		case "error":
			errored++
			fmt.Fprintf(os.Stderr, "  ✗ %s (%s)\n", r.Source, r.Error)
		}
	}

	latencyMs := time.Since(started).Milliseconds()
	summary := fmt.Sprintf("session-index: %d indexed, %d skipped, %d errors in %dms",
		indexed, skipped, errored, latencyMs)
	log.Log(summary)
	fmt.Fprintln(os.Stderr, summary)

	metricsPath := paths.MetricsFile()
	if metricsPath != "" {
		_ = metrics.Record(metricsPath, metrics.Event{
			Hook:      "session-index",
			Action:    "batch-index",
			Category:  "ctxmode",
			Detail:    fmt.Sprintf("indexed=%d skipped=%d errors=%d", indexed, skipped, errored),
			LatencyMs: latencyMs,
		})
	}

	out, _ := json.Marshal(results)
	fmt.Println(string(out))

	if errored > 0 {
		return fmt.Errorf("%d targets failed to index", errored)
	}
	return nil
}
