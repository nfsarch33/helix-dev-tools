package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/nfsarch33/cursor-tools/internal/agentrace"
	"github.com/spf13/cobra"
)

var agentraceSearchFlags struct {
	logsDir string
	limit   int
	asJSON  bool
}

var agentraceSearchCmd = &cobra.Command{
	Use:   "agentrace-search <query>",
	Short: "Full-text search across agentrace NDJSON logs",
	Long:  "Builds an in-memory TF-IDF index over all .ndjson files in ~/logs/runx/ and returns ranked results.",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := agentraceSearchFlags.logsDir
		if dir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("resolve home: %w", err)
			}
			dir = filepath.Join(home, "logs", "runx")
		}
		query := args[0]
		return runAgentraceSearch(cmd.OutOrStdout(), dir, query,
			agentraceSearchFlags.limit, agentraceSearchFlags.asJSON)
	},
}

func init() {
	agentraceSearchCmd.Flags().StringVar(&agentraceSearchFlags.logsDir, "logs-dir", "", "NDJSON logs directory (default: ~/logs/runx)")
	agentraceSearchCmd.Flags().IntVar(&agentraceSearchFlags.limit, "limit", 20, "Maximum results to return")
	agentraceSearchCmd.Flags().BoolVar(&agentraceSearchFlags.asJSON, "json", false, "Output results as JSON")
}

func runAgentraceSearch(w io.Writer, dir, query string, limit int, asJSON bool) error {
	idx, err := agentrace.BuildIndex(dir)
	if err != nil {
		return fmt.Errorf("agentrace-search: build index: %w", err)
	}

	results := idx.Search(query, limit)
	if len(results) == 0 {
		_, _ = fmt.Fprintf(w, "agentrace-search: no results for %q\n", query)
		return nil
	}

	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	}

	_, _ = fmt.Fprintf(w, "agentrace-search: %d results for %q\n\n", len(results), query)
	for i, r := range results {
		_, _ = fmt.Fprintf(w, "[%d] (%.2f) %s\n    %s\n\n", i+1, r.Score, r.File, r.Line)
	}
	return nil
}
