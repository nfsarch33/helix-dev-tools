package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nfsarch33/helix-dev-tools/internal/platform/evalrecord"
	"github.com/spf13/cobra"
)

func newEvalRecordCmd() *cobra.Command {
	var (
		branch        string
		ticketID      string
		agentID       string
		testsRun      int
		testsPassed   int
		sentruxBefore int
		sentruxAfter  int
		mcpHealthy    bool
		elapsed       float64
		evidence      string
	)

	c := &cobra.Command{
		Use:   "record",
		Short: "Record a QA evaluation result for a sprint pair",
		RunE: func(cmd *cobra.Command, args []string) error {
			r := evalrecord.New(branch, ticketID, agentID)
			r.SetTests(testsRun, testsPassed)
			r.SetSentrux(sentruxBefore, sentruxAfter)
			r.SetMCPHealth(mcpHealthy)
			r.SetElapsed(time.Duration(elapsed * float64(time.Second)))
			r.SetEvidence(evidence)
			r.Evaluate()

			logPath := filepath.Join(os.Getenv("HOME"), "logs", "runx", "eval-records.ndjson")
			if err := os.MkdirAll(filepath.Dir(logPath), 0700); err != nil {
				return fmt.Errorf("create log dir: %w", err)
			}
			f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
			if err != nil {
				return fmt.Errorf("open log: %w", err)
			}
			defer f.Close()

			data, _ := json.Marshal(r)
			fmt.Fprintf(f, "%s\n", data)

			fmt.Fprintf(cmd.OutOrStdout(), "verdict=%s risk=%d evospine_candidate=%v\n",
				r.Verdict, r.RiskScore, r.IsEvoSpineCandidate())
			return nil
		},
	}

	c.Flags().StringVar(&branch, "branch", "", "Branch name")
	c.Flags().StringVar(&ticketID, "ticket", "", "Ticket ID")
	c.Flags().StringVar(&agentID, "agent", "", "Agent ID")
	c.Flags().IntVar(&testsRun, "tests-run", 0, "Number of tests executed")
	c.Flags().IntVar(&testsPassed, "tests-passed", 0, "Number of tests passed")
	c.Flags().IntVar(&sentruxBefore, "sentrux-before", 0, "Sentrux score before change")
	c.Flags().IntVar(&sentruxAfter, "sentrux-after", 0, "Sentrux score after change")
	c.Flags().BoolVar(&mcpHealthy, "mcp-healthy", true, "Whether MCP tools are healthy")
	c.Flags().Float64Var(&elapsed, "elapsed-sec", 0, "Elapsed seconds")
	c.Flags().StringVar(&evidence, "evidence", "", "Link to evidence (commit SHA, PR URL)")

	return c
}

func init() {
	evalCmd.AddCommand(newEvalRecordCmd())
}
