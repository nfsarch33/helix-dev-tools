package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/helix-dev-tools/internal/noshellleaksync"
)

var guardNoShellLeakSyncCmd = &cobra.Command{
	Use:   "guard-no-shell-leak-sync",
	Short: "beforeReadAgent: SHA-verify and resync no-shell-leak rule across 14 mirror repos",
	Long: `Reads the canonical no-shell-leak rule from
$HOME/Code/global-kb/cursor-config/rules/no-shell-leak.mdc and ensures every
personal-repo mirror at <repo>/.cursor/rules/no-shell-leak.mdc matches its
SHA-256.

Drifted mirrors are rewritten in place. Missing repos and missing mirror
files are reported but treated as opt-out, not error.

Outputs a single JSON object summarising the run, suitable for Cursor hook
ingestion (stdout JSON contract per .cursor/hooks.json).

Wired in v299 D6. The 14 mirror repos are the same active personal repos
covered by the runx no-shell-leak alias inventory; public mirror repos and
dormant repos are intentionally excluded.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolve $HOME: %w", err)
		}
		syncer := noshellleaksync.NewSyncer(homeDir)
		results, err := syncer.Run()
		if err != nil {
			return err
		}
		summary := map[string]any{
			"hook":         "guard-no-shell-leak-sync",
			"version":      "v299-d6",
			"mirror_count": len(results),
			"results":      results,
			"in_sync":      countAction(results, noshellleaksync.ActionInSync),
			"resynced":     countAction(results, noshellleaksync.ActionResynced),
			"repo_missing": countAction(results, noshellleaksync.ActionRepoMissing),
			"mirror_skip":  countAction(results, noshellleaksync.ActionMirrorMissing),
			"errors":       countAction(results, noshellleaksync.ActionError),
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(summary)
	},
}

func countAction(results []noshellleaksync.SyncResult, want noshellleaksync.Action) int {
	n := 0
	for _, r := range results {
		if r.Action == want {
			n++
		}
	}
	return n
}
