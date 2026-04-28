package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/cursor-tools/internal/clilog"
	"github.com/nfsarch33/cursor-tools/internal/config"
	"github.com/nfsarch33/cursor-tools/internal/mem0outbox"
)

var mem0OutboxFlags struct {
	root           string
	apiKey         string
	baseURL        string
	mcpJSON        string
	batchSize      int
	maxIterations  int
	flushIntervalS int
	tail           int
	once           bool
	dryRun         bool
}

var mem0OutboxCmd = &cobra.Command{
	Use:   "mem0-outbox",
	Short: "Drain the durable Mem0 outbox into the Mem0 hot layer",
	Long: "Reads NDJSON capsules from the Mem0 outbox and POSTs them to Mem0 v1 /memories.\n" +
		"Cursor advances on each successful POST so a 429 freezes progress at the failed line.\n" +
		"Use --once for an opportunistic flush from a cron job; omit it to run a long-lived flush daemon.",
	RunE:          runMem0Outbox,
	SilenceUsage:  true,
	SilenceErrors: false,
}

func init() {
	defaultRoot := filepath.Join(os.Getenv("HOME"), ".cursor", "mem0-outbox")
	mem0OutboxCmd.Flags().StringVar(&mem0OutboxFlags.root, "root", defaultRoot, "Outbox root directory (must contain pending.jsonl + cursor)")
	mem0OutboxCmd.Flags().StringVar(&mem0OutboxFlags.apiKey, "api-key", "", "Mem0 API key (defaults to env or ~/.cursor/mcp.json)")
	mem0OutboxCmd.Flags().StringVar(&mem0OutboxFlags.baseURL, "base-url", "https://api.mem0.ai", "Mem0 base URL (override only for smoke / staging)")
	mem0OutboxCmd.Flags().StringVar(&mem0OutboxFlags.mcpJSON, "mcp-json", config.DefaultPaths().CursorMCPConfig(), "Path to Cursor MCP config for resolving Mem0 credentials")
	mem0OutboxCmd.Flags().IntVar(&mem0OutboxFlags.batchSize, "batch-size", 50, "Maximum capsules drained in one Flush call")
	mem0OutboxCmd.Flags().IntVar(&mem0OutboxFlags.maxIterations, "max-iterations", 0, "When running as a daemon, stop after this many flush iterations (0 = infinite)")
	mem0OutboxCmd.Flags().IntVar(&mem0OutboxFlags.flushIntervalS, "interval-seconds", 30, "Daemon flush cadence between batches")
	mem0OutboxCmd.Flags().IntVar(&mem0OutboxFlags.tail, "tail", 0, "Print the last N capsules and exit (read-only; no flushing)")
	mem0OutboxCmd.Flags().BoolVar(&mem0OutboxFlags.once, "once", false, "Run a single flush iteration and exit")
	mem0OutboxCmd.Flags().BoolVar(&mem0OutboxFlags.dryRun, "dry-run", false, "Read pending capsules and report counts without calling Mem0")
}

func runMem0Outbox(_ *cobra.Command, _ []string) error {
	pendingPath := filepath.Join(mem0OutboxFlags.root, "pending.jsonl")
	cursorPath := filepath.Join(mem0OutboxFlags.root, "cursor")

	if mem0OutboxFlags.tail > 0 {
		reader := mem0outbox.NewReader(pendingPath, cursorPath)
		caps, err := reader.Tail(mem0OutboxFlags.tail)
		if err != nil {
			return fmt.Errorf("tail: %w", err)
		}
		clilog.Header(fmt.Sprintf("cursor-tools mem0-outbox tail (n=%d)", mem0OutboxFlags.tail))
		for _, c := range caps {
			fmt.Printf("  - id=%s app_id=%s user_id=%s text=%q\n", c.ID, c.AppID, c.UserID, abbreviate(c.Text, 80))
		}
		return nil
	}

	if mem0OutboxFlags.dryRun {
		stat, err := os.Stat(pendingPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				fmt.Println("  no pending.jsonl yet (outbox empty)")
				return nil
			}
			return err
		}
		fmt.Printf("  pending bytes: %d (path=%s)\n", stat.Size(), pendingPath)
		fmt.Printf("  flush would batch up to %d capsules per call (interval=%ds)\n", mem0OutboxFlags.batchSize, mem0OutboxFlags.flushIntervalS)
		return nil
	}

	apiKey := strings.TrimSpace(mem0OutboxFlags.apiKey)
	if apiKey == "" {
		cfg, err := resolveMem0AuditConfig(config.DefaultPaths(), struct {
			apiKey         string
			userID         string
			appID          string
			mcpJSON        string
			pageSize       int
			export         string
			strict         bool
			syncProvenance bool
			dryRun         bool
		}{
			apiKey:  "",
			userID:  "",
			appID:   "cursor-global-kb",
			mcpJSON: mem0OutboxFlags.mcpJSON,
		})
		if err != nil {
			return err
		}
		apiKey = cfg.APIKey
	}
	client := mem0outbox.NewMem0Client(mem0OutboxFlags.baseURL, apiKey)
	flusher := &mem0outbox.Flusher{
		PendingPath: pendingPath,
		CursorPath:  cursorPath,
		Client:      client,
		BatchSize:   mem0OutboxFlags.batchSize,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	iterations := 0
	for {
		report, err := flusher.Flush(ctx)
		iterations++
		switch {
		case errors.Is(err, mem0outbox.ErrRateLimited):
			fmt.Printf("  iteration=%d flushed=%d retry_after=%s\n", iterations, report.Flushed, report.RetryAfter)
			if mem0OutboxFlags.once {
				return err
			}
			sleep(ctx, report.RetryAfter)
		case err != nil:
			return err
		default:
			fmt.Printf("  iteration=%d flushed=%d skipped=%d\n", iterations, report.Flushed, report.Skipped)
			if mem0OutboxFlags.once {
				return nil
			}
			sleep(ctx, time.Duration(mem0OutboxFlags.flushIntervalS)*time.Second)
		}
		if mem0OutboxFlags.maxIterations > 0 && iterations >= mem0OutboxFlags.maxIterations {
			return nil
		}
	}
}

func sleep(ctx context.Context, d time.Duration) {
	if d <= 0 {
		return
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

func abbreviate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
