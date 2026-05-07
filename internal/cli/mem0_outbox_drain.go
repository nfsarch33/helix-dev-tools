package cli

import (
	"context"
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

var mem0DrainFlags struct {
	root      string
	apiKey    string
	ossAPIKey string
	baseURL   string
	ossURL    string
	mcpJSON   string
	batchSize int
	dryRun    bool
}

var mem0DrainCmd = &cobra.Command{
	Use:   "mem0-drain",
	Short: "Drain buffered outbox writes to both managed and OSS after quota reset",
	Long: "On quota reset, drain buffered NDJSON capsules to both managed and OSS Mem0 backends.\n" +
		"Each capsule is pushed exactly once to each backend (idempotent under retry).\n" +
		"Use --dry-run to count pending capsules without calling either backend.\n\n" +
		"BLOCKER: Managed Mem0 quota blocked until 2026-05-11. Use --dry-run until then.",
	RunE:          runMem0Drain,
	SilenceUsage:  true,
	SilenceErrors: false,
}

func init() {
	defaultRoot := filepath.Join(os.Getenv("HOME"), ".cursor", "mem0-outbox")
	mem0DrainCmd.Flags().StringVar(&mem0DrainFlags.root, "root", defaultRoot, "Outbox root directory")
	mem0DrainCmd.Flags().StringVar(&mem0DrainFlags.apiKey, "api-key", "", "Managed Mem0 API key (defaults to env or MCP config)")
	mem0DrainCmd.Flags().StringVar(&mem0DrainFlags.ossAPIKey, "oss-api-key", "", "OSS Mem0 API key")
	mem0DrainCmd.Flags().StringVar(&mem0DrainFlags.baseURL, "base-url", "https://api.mem0.ai", "Managed Mem0 base URL")
	mem0DrainCmd.Flags().StringVar(&mem0DrainFlags.ossURL, "oss-url", "http://127.0.0.1:8083", "OSS Mem0 base URL")
	mem0DrainCmd.Flags().StringVar(&mem0DrainFlags.mcpJSON, "mcp-json", config.DefaultPaths().CursorMCPConfig(), "Path to Cursor MCP config")
	mem0DrainCmd.Flags().IntVar(&mem0DrainFlags.batchSize, "batch-size", 50, "Maximum capsules drained per call")
	mem0DrainCmd.Flags().BoolVar(&mem0DrainFlags.dryRun, "dry-run", false, "Count pending capsules without pushing")
}

func runMem0Drain(_ *cobra.Command, _ []string) error {
	clilog.Header("cursor-tools mem0-drain (quota-reset)")

	pendingPath := filepath.Join(mem0DrainFlags.root, "pending.jsonl")
	cursorPath := filepath.Join(mem0DrainFlags.root, "drain-cursor")

	if mem0DrainFlags.dryRun {
		fmt.Println("  mode: --dry-run (no pushes)")
	}

	apiKey := strings.TrimSpace(mem0DrainFlags.apiKey)
	if apiKey == "" && !mem0DrainFlags.dryRun {
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
			mcpJSON: mem0DrainFlags.mcpJSON,
			appID:   "cursor-global-kb",
		})
		if err != nil {
			return fmt.Errorf("resolve managed API key: %w", err)
		}
		apiKey = cfg.APIKey
	}

	var managed, oss mem0outbox.Mem0Pusher
	if !mem0DrainFlags.dryRun {
		managed = mem0outbox.NewMem0Client(mem0DrainFlags.baseURL, apiKey)
		oss = mem0outbox.NewMem0Client(mem0DrainFlags.ossURL, mem0DrainFlags.ossAPIKey)
	}

	drainer := &mem0outbox.QuotaDrainer{
		PendingPath: pendingPath,
		CursorPath:  cursorPath,
		Managed:     managed,
		OSS:         oss,
		BatchSize:   mem0DrainFlags.batchSize,
		DryRun:      mem0DrainFlags.dryRun,
	}

	ctx := context.Background()
	report, err := drainer.Drain(ctx)
	if err != nil {
		return err
	}

	if mem0DrainFlags.dryRun {
		fmt.Printf("  pending:   %d capsules to drain\n", report.Pending)
		fmt.Printf("  skipped:   %d (corrupt lines)\n", report.Skipped)
	} else {
		fmt.Printf("  drained:   %d capsules (managed + OSS)\n", report.Drained)
		fmt.Printf("  skipped:   %d\n", report.Skipped)
	}
	fmt.Printf("  duration:  %s\n", report.Duration.Round(time.Millisecond))

	return nil
}
