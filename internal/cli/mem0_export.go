package cli

import (
	"fmt"
	"os"

	"github.com/nfsarch33/cursor-tools/internal/mem0export"
	"github.com/spf13/cobra"
)

var mem0ExportFlags struct {
	all    bool
	output string
}

var mem0ExportCmd = &cobra.Command{
	Use:   "mem0-export",
	Short: "Export all Mem0 memories to NDJSON",
	Long: `Fetches every memory from Mem0 OSS (paginated) and writes one JSON
object per line to the output file. Requires MEM0_BASE_URL and optionally
MEM0_API_KEY environment variables.`,
	RunE: runMem0Export,
}

func init() {
	mem0ExportCmd.Flags().BoolVar(&mem0ExportFlags.all, "all", false, "Export all memories (required)")
	mem0ExportCmd.Flags().StringVarP(&mem0ExportFlags.output, "output", "o", "", "Output file path (default: stdout)")
}

func runMem0Export(_ *cobra.Command, _ []string) error {
	if !mem0ExportFlags.all {
		return fmt.Errorf("--all flag is required")
	}

	baseURL := os.Getenv("MEM0_BASE_URL")
	if baseURL == "" {
		return fmt.Errorf("MEM0_BASE_URL environment variable is required")
	}

	exp := &mem0export.Exporter{
		BaseURL: baseURL,
		APIKey:  os.Getenv("MEM0_API_KEY"),
	}

	var out *os.File
	if mem0ExportFlags.output != "" {
		f, err := os.Create(mem0ExportFlags.output)
		if err != nil {
			return fmt.Errorf("create output file: %w", err)
		}
		defer f.Close()
		out = f
	} else {
		out = os.Stdout
	}

	n, err := exp.Export(out)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Exported %d memories\n", n)
	return nil
}
