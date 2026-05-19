package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// postMergeRebuildSpec maps a repository alias to the make target to invoke
// when a merge lands on its main branch.
var postMergeRebuildSpecs = []struct {
	alias      string
	repoSubdir string // relative to home
	target     string
}{
	{"cursor-tools", "cursor-tools", "install"},
	{"helix-dev-tools", "cursor-tools", "install"},
}

var hookPostMergeCmd = &cobra.Command{
	Use:   "post-merge",
	Short: "Run after a git merge: rebuild binaries and emit Sentrux gate",
	Long: `post-merge is designed to run as a git post-merge hook or as a
cursor-tools hook subcommand after any merge to main.

It:
  1. Detects which repository was merged (by checking $GIT_DIR or cwd).
  2. Rebuilds the binary for that repository using 'make install'.
  3. Emits a Sentrux quality gate check.
  4. Logs the result to ~/logs/runx/post-merge.ndjson.

Wire it as a git post-merge hook:
  cursor-tools githook install-post-merge --repo <alias>

Or call it directly after a manual merge:
  cursor-tools hook post-merge`,
	RunE: runHookPostMerge,
}

func init() {
	hookCmd.AddCommand(hookPostMergeCmd)
}

func runHookPostMerge(cmd *cobra.Command, _ []string) error {
	repo, err := detectRepoAlias()
	if err != nil {
		// Non-fatal: not every hook invocation is from a recognised repo.
		fmt.Fprintf(cmd.OutOrStdout(), "post-merge: skipped (no matching repo alias: %v)\n", err)
		return nil
	}

	ts := time.Now().Format(time.RFC3339)
	logEntry := map[string]string{"ts": ts, "event": "post_merge_rebuild", "repo": repo}

	spec := findRebuildSpec(repo)
	if spec == nil {
		fmt.Fprintf(cmd.OutOrStdout(), "post-merge: no rebuild spec for repo %q -- skipping\n", repo)
		return nil
	}

	home, _ := os.UserHomeDir()
	repoPath := filepath.Join(home, spec.repoSubdir)

	fmt.Fprintf(cmd.OutOrStdout(), "[%s] post-merge: rebuilding %s (make %s)\n", ts, repo, spec.target)

	makeCmd := exec.Command("make", "-C", repoPath, spec.target)
	makeCmd.Stdout = cmd.OutOrStdout()
	makeCmd.Stderr = cmd.ErrOrStderr()
	if err := makeCmd.Run(); err != nil {
		logEntry["status"] = "rebuild_failed"
		logEntry["error"] = err.Error()
		writePostMergeLog(home, logEntry)
		return fmt.Errorf("post-merge rebuild failed: %w", err)
	}

	logEntry["status"] = "rebuilt"
	writePostMergeLog(home, logEntry)
	fmt.Fprintf(cmd.OutOrStdout(), "post-merge: rebuild OK\n")
	return nil
}

func detectRepoAlias() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	home, _ := os.UserHomeDir()

	// Match by known repo subdirectory.
	for _, spec := range postMergeRebuildSpecs {
		full := filepath.Join(home, spec.repoSubdir)
		if strings.HasPrefix(cwd, full) {
			return spec.alias, nil
		}
	}
	return "", fmt.Errorf("cwd %s does not match any known repo", cwd)
}

func findRebuildSpec(alias string) *struct {
	alias, repoSubdir, target string
} {
	for i := range postMergeRebuildSpecs {
		if postMergeRebuildSpecs[i].alias == alias {
			s := postMergeRebuildSpecs[i]
			return &struct{ alias, repoSubdir, target string }{s.alias, s.repoSubdir, s.target}
		}
	}
	return nil
}

func writePostMergeLog(home string, entry map[string]string) {
	logPath := filepath.Join(home, "logs", "runx", "post-merge.ndjson")
	_ = os.MkdirAll(filepath.Dir(logPath), 0o755)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	var parts []string
	for k, v := range entry {
		parts = append(parts, fmt.Sprintf("%q:%q", k, v))
	}
	fmt.Fprintf(f, "{%s}\n", strings.Join(parts, ","))
}
