package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/helix-dev-tools/internal/clilog"
	"github.com/nfsarch33/helix-dev-tools/internal/config"
	"github.com/nfsarch33/helix-dev-tools/internal/mem0"
)

var mem0CanaryFlags struct {
	ossURL     string
	managedURL string
	ossAPIKey  string
	apiKey     string
	mcpJSON    string
	flipPct    int
	logPath    string
	queries    string
	timeout    int
}

var mem0CanaryCmd = &cobra.Command{
	Use:   "mem0-canary",
	Short: "Run the Mem0 read-flip canary comparing OSS and managed top-3 overlap",
	Long: "Routes a configurable percentage of reads to OSS first with managed as fallback.\n" +
		"Records top-3 search result overlap per query in NDJSON for migration readiness assessment.",
	RunE:          runMem0Canary,
	SilenceUsage:  true,
	SilenceErrors: false,
}

func init() {
	defaultLog := filepath.Join(os.Getenv("HOME"), "logs", "runx", "mem0-readflip.ndjson")
	mem0CanaryCmd.Flags().StringVar(&mem0CanaryFlags.ossURL, "oss-url", "http://127.0.0.1:8083", "Mem0 OSS endpoint")
	mem0CanaryCmd.Flags().StringVar(&mem0CanaryFlags.managedURL, "managed-url", "https://api.mem0.ai", "Mem0 managed endpoint")
	mem0CanaryCmd.Flags().StringVar(&mem0CanaryFlags.ossAPIKey, "oss-api-key", "", "API key for OSS endpoint")
	mem0CanaryCmd.Flags().StringVar(&mem0CanaryFlags.apiKey, "api-key", "", "Managed Mem0 API key (defaults to env or MCP config)")
	mem0CanaryCmd.Flags().StringVar(&mem0CanaryFlags.mcpJSON, "mcp-json", config.DefaultPaths().CursorMCPConfig(), "Path to Cursor MCP config for resolving Mem0 credentials")
	mem0CanaryCmd.Flags().IntVar(&mem0CanaryFlags.flipPct, "flip-pct", 10, "Percentage of reads flipped to OSS (0-100)")
	mem0CanaryCmd.Flags().StringVar(&mem0CanaryFlags.logPath, "log", defaultLog, "NDJSON log path for overlap entries")
	mem0CanaryCmd.Flags().StringVar(&mem0CanaryFlags.queries, "queries", "cursor rules,evoloop status,mem0 health,workspace doctor", "Comma-separated search queries")
	mem0CanaryCmd.Flags().IntVar(&mem0CanaryFlags.timeout, "timeout", 10, "Per-query timeout in seconds")
}

func runMem0Canary(_ *cobra.Command, _ []string) error {
	clilog.Header("cursor-tools mem0-canary (read-flip)")

	queries := strings.Split(mem0CanaryFlags.queries, ",")
	for i := range queries {
		queries[i] = strings.TrimSpace(queries[i])
	}

	apiKey := strings.TrimSpace(mem0CanaryFlags.apiKey)
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
			mcpJSON: mem0CanaryFlags.mcpJSON,
			appID:   "cursor-global-kb",
		})
		if err != nil {
			return fmt.Errorf("resolve managed API key: %w", err)
		}
		apiKey = cfg.APIKey
	}

	timeout := time.Duration(mem0CanaryFlags.timeout) * time.Second

	oss := &mem0.HTTPSearcher{
		Endpoint: mem0CanaryFlags.ossURL,
		APIKey:   mem0CanaryFlags.ossAPIKey,
		Timeout:  timeout,
	}
	managed := &mem0.HTTPSearcher{
		Endpoint: mem0CanaryFlags.managedURL,
		APIKey:   apiKey,
		Timeout:  timeout,
	}

	rf := &mem0.ReadFlip{
		OSS:     oss,
		Managed: managed,
		FlipPct: mem0CanaryFlags.flipPct,
		LogPath: mem0CanaryFlags.logPath,
		Queries: queries,
		Timeout: timeout,
	}

	ctx := context.Background()
	report, err := rf.Run(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("  queries:   %d\n", report.TotalQueries)
	fmt.Printf("  flipped:   %d\n", report.FlippedToOSS)
	fmt.Printf("  overlap:   %.2f%%\n", report.AvgOverlap*100)
	fmt.Printf("  duration:  %s\n", report.Duration.Round(time.Millisecond))
	fmt.Printf("  log:       %s\n", mem0CanaryFlags.logPath)

	return nil
}
