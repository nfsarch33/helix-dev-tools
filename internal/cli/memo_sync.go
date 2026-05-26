package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/nfsarch33/helix-dev-tools/internal/memosync"
	"github.com/spf13/cobra"
)

var (
	memoGlobalKBRoot string
	memoRoot         string
)

var memoCmd = &cobra.Command{
	Use:   "memo",
	Short: "Memo backup management",
}

var memoSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync critical global-kb content to the memo backup repo",
	Long: `Copies changed files from global-kb (SOPs, ADRs, config templates,
handoffs) into ~/memo and commits with a timestamped message.
Does NOT push — remote setup is left to the operator.`,
	RunE: runMemoSync,
}

func init() {
	home, _ := os.UserHomeDir()
	defaultGK := filepath.Join(home, "Code", "global-kb")
	defaultMemo := filepath.Join(home, "memo")

	memoSyncCmd.Flags().StringVar(&memoGlobalKBRoot, "global-kb", defaultGK, "global-kb repository root")
	memoSyncCmd.Flags().StringVar(&memoRoot, "memo", defaultMemo, "memo backup repository root")
	memoCmd.AddCommand(memoSyncCmd)
}

// gitRunner abstracts git operations for testability.
var gitRunner func(dir string, args ...string) error = execGit

func execGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runMemoSync(cmd *cobra.Command, _ []string) error {
	out := cmd.OutOrStdout()
	return doMemoSync(out, memoGlobalKBRoot, memoRoot)
}

func doMemoSync(out io.Writer, gkRoot, mRoot string) error {
	res, err := memosync.Sync(gkRoot, mRoot)
	if err != nil {
		return fmt.Errorf("memo sync: %w", err)
	}

	for _, e := range res.Errors {
		fmt.Fprintf(out, "[WARN] %v\n", e)
	}

	if len(res.Copied) == 0 {
		fmt.Fprintln(out, "memo sync: no changes detected")
		return nil
	}

	for _, f := range res.Copied {
		fmt.Fprintf(out, "[COPIED] %s\n", f)
	}
	fmt.Fprintf(out, "memo sync: %d copied, %d skipped, %d errors\n",
		len(res.Copied), res.Skipped, len(res.Errors))

	now := time.Now()
	if modified, err := memosync.UpdateReadme(mRoot, now); err != nil {
		fmt.Fprintf(out, "[WARN] update README: %v\n", err)
	} else if modified {
		fmt.Fprintln(out, "[UPDATED] README.md last-sync timestamp")
	}

	commitMsg := memosync.CommitMessage()
	if err := gitRunner(mRoot, "add", "-A"); err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	if err := gitRunner(mRoot, "commit", "-m", commitMsg); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	fmt.Fprintf(out, "memo sync: committed %q\n", commitMsg)
	return nil
}
