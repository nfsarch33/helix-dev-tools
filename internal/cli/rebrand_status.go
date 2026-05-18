package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// rebrandSOPSummary holds parsed counts from the rebranding SOP markdown tables.
type rebrandSOPSummary struct {
	Done     int
	Pending  int
	Deferred int
}

func (s rebrandSOPSummary) Total() int { return s.Done + s.Pending + s.Deferred }

// parseRebrandSOP reads a markdown file and counts rows whose last | -delimited
// cell contains DONE, PENDING, or DEFERRED (case-sensitive).
func parseRebrandSOP(path string) (rebrandSOPSummary, error) {
	f, err := os.Open(path)
	if err != nil {
		return rebrandSOPSummary{}, err
	}
	defer f.Close()

	var summary rebrandSOPSummary
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "|") {
			continue
		}
		// Skip separator rows (---|---).
		if strings.Contains(line, "---") {
			continue
		}
		cells := strings.Split(line, "|")
		// cells[0] is empty (leading |), cells[len-1] is empty (trailing |).
		for _, cell := range cells {
			cell = strings.TrimSpace(cell)
			switch cell {
			case "DONE":
				summary.Done++
			case "PENDING":
				summary.Pending++
			case "DEFERRED":
				summary.Deferred++
			}
		}
	}
	return summary, scanner.Err()
}

// runxAliasMigrationStatus returns "PENDING" when the runx override_remote
// diff file exists (waiting for operator to apply), "DONE" when it is absent
// (operator has applied and removed it).
func runxAliasMigrationStatus(diffPath string) string {
	if _, err := os.Stat(diffPath); err == nil {
		return "PENDING"
	}
	return "DONE"
}

// buildRebrandDashboard constructs a Markdown dashboard table summarising all
// rebrand milestones. sopPath is the rebranding SOP; diffPath is the runx
// pending-change diff file; asOf is an ISO 8601 timestamp string.
func buildRebrandDashboard(sopPath, diffPath, asOf string) string {
	summary, _ := parseRebrandSOP(sopPath)
	runxState := runxAliasMigrationStatus(diffPath)

	var b strings.Builder
	b.WriteString("# Helixon Rebranding Status Dashboard\n\n")
	b.WriteString("**As of**: " + asOf + "\n\n")
	b.WriteString("| Area | Done | Pending | Deferred |\n")
	b.WriteString("|---|---|---|---|\n")
	b.WriteString(fmt.Sprintf("| GitHub Renames | %d | %d | %d |\n",
		summary.Done, summary.Pending, summary.Deferred))
	b.WriteString(fmt.Sprintf("| runx Alias Migration | %s | | |\n",
		runxState))
	b.WriteString("\n")
	if summary.Pending > 0 || runxState == "PENDING" {
		b.WriteString("Items still PENDING. See SOP for details.\n")
	} else {
		b.WriteString("All tracked items DONE or DEFERRED.\n")
	}
	return b.String()
}

var (
	rebrandStatusSOPPath       string
	rebrandStatusDashboard     bool
	rebrandStatusDiffPath      string
)

var rebrandStatusCmd = &cobra.Command{
	Use:          "status",
	Short:        "Report Helixon rebranding progress from the coordination SOP",
	SilenceUsage: true,
	RunE:         runRebrandStatus,
}

func init() {
	home := os.Getenv("HOME")
	defaultSOP := filepath.Join(home, "Code", "global-kb", "sop", "helixon-rebranding-coordination.md")
	defaultDiff := filepath.Join(home, "Code", "global-kb", "pending-changes", "runx-override-remote.yaml.diff")
	rebrandStatusCmd.Flags().StringVar(&rebrandStatusSOPPath, "sop", defaultSOP, "Path to helixon-rebranding-coordination.md")
	rebrandStatusCmd.Flags().BoolVar(&rebrandStatusDashboard, "dashboard", false, "Emit full Markdown dashboard")
	rebrandStatusCmd.Flags().StringVar(&rebrandStatusDiffPath, "diff-path", defaultDiff, "Path to runx override_remote diff file")
	rebrandCmd.AddCommand(rebrandStatusCmd)
}

// newRebrandStatusCmd returns a cobra.Command pre-configured with a custom SOP
// path. Used by tests to avoid filesystem side-effects.
func newRebrandStatusCmd(sopPath string) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "status",
		Short:        "Report rebranding progress",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return emitRebrandStatus(cmd, sopPath)
		},
	}
	return cmd
}

func runRebrandStatus(cmd *cobra.Command, _ []string) error {
	if rebrandStatusDashboard {
		w := cmd.OutOrStdout()
		fmt.Fprint(w, buildRebrandDashboard(rebrandStatusSOPPath, rebrandStatusDiffPath, "now"))
		return nil
	}
	return emitRebrandStatus(cmd, rebrandStatusSOPPath)
}

func emitRebrandStatus(cmd *cobra.Command, sopPath string) error {
	summary, err := parseRebrandSOP(sopPath)
	if err != nil {
		return fmt.Errorf("read SOP: %w", err)
	}

	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Helixon Rebranding Status\n")
	fmt.Fprintf(w, "=========================\n")
	fmt.Fprintf(w, "  DONE:     %d\n", summary.Done)
	fmt.Fprintf(w, "  PENDING:  %d\n", summary.Pending)
	fmt.Fprintf(w, "  DEFERRED: %d\n", summary.Deferred)
	fmt.Fprintf(w, "  TOTAL:    %d\n", summary.Total())

	if summary.Pending > 0 {
		fmt.Fprintf(w, "\n%d item(s) still PENDING. See SOP: %s\n", summary.Pending, sopPath)
	} else {
		fmt.Fprintf(w, "\nAll tracked items DONE or DEFERRED.\n")
	}
	return nil
}
