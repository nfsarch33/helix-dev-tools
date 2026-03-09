package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nfsarch33/cursor-tools/internal/worktree"
	"github.com/spf13/cobra"
)

var worktreeFromBranch string

var worktreeCmd = &cobra.Command{
	Use:   "worktree",
	Short: "Manage git worktrees for parallel agent execution",
	Long:  "Create, list, and clean up git worktrees. Automates .gitignore verification and .env file syncing.",
}

var worktreeCreateCmd = &cobra.Command{
	Use:   "create <branch>",
	Short: "Create a new worktree with safety checks",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		branch := args[0]

		repoRoot, err := gitRepoRoot()
		if err != nil {
			return err
		}

		if err := worktree.EnsureGitignore(repoRoot, worktree.WorktreeDir+"/"); err != nil {
			return fmt.Errorf("gitignore: %w", err)
		}
		fmt.Println("  OK  .gitignore verified")

		wtDir := filepath.Join(repoRoot, worktree.WorktreeDir, branch)
		gitArgs := []string{"worktree", "add", wtDir, "-b", branch}
		if worktreeFromBranch != "" {
			gitArgs = append(gitArgs, worktreeFromBranch)
		}

		out, err := exec.Command("git", gitArgs...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("git worktree add: %s\n%w", string(out), err)
		}
		fmt.Printf("  OK  worktree created at %s\n", wtDir)

		copied, cpErr := worktree.CopyEnvFiles(repoRoot, wtDir)
		if cpErr != nil {
			fmt.Fprintf(os.Stderr, "  WARN  env copy: %v\n", cpErr)
		} else if len(copied) > 0 {
			fmt.Printf("  OK  copied %d env files: %s\n", len(copied), strings.Join(copied, ", "))
		} else {
			fmt.Println("  INFO  no .env files to copy")
		}

		return nil
	},
}

var worktreeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all active worktrees",
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := exec.Command("git", "worktree", "list").CombinedOutput()
		if err != nil {
			return fmt.Errorf("git worktree list: %s\n%w", string(out), err)
		}

		entries := worktree.ParseWorktreeList(string(out))
		if len(entries) == 0 {
			fmt.Println("No worktrees found.")
			return nil
		}

		fmt.Printf("Active worktrees (%d):\n", len(entries))
		for _, e := range entries {
			fmt.Printf("  %-50s [%s] %s\n", e.Path, e.Branch, e.Commit[:7])
		}
		return nil
	},
}

var worktreeCleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Remove inactive worktrees and prune references",
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := exec.Command("git", "worktree", "list").CombinedOutput()
		if err != nil {
			return fmt.Errorf("git worktree list: %s\n%w", string(out), err)
		}

		entries := worktree.ParseWorktreeList(string(out))

		repoRoot, rootErr := gitRepoRoot()
		if rootErr != nil {
			return rootErr
		}

		wtBase := filepath.Join(repoRoot, worktree.WorktreeDir)
		removed := 0
		for _, e := range entries {
			if !strings.HasPrefix(e.Path, wtBase) {
				continue
			}
			rmOut, rmErr := exec.Command("git", "worktree", "remove", e.Path).CombinedOutput()
			if rmErr != nil {
				fmt.Fprintf(os.Stderr, "  WARN  remove %s: %s\n", e.Branch, strings.TrimSpace(string(rmOut)))
				continue
			}
			fmt.Printf("  OK  removed %s [%s]\n", e.Path, e.Branch)
			removed++
		}

		pruneOut, pruneErr := exec.Command("git", "worktree", "prune").CombinedOutput()
		if pruneErr != nil {
			fmt.Fprintf(os.Stderr, "  WARN  prune: %s\n", string(pruneOut))
		}

		if removed == 0 {
			fmt.Println("No linked worktrees to clean up.")
		} else {
			fmt.Printf("Cleaned up %d worktrees.\n", removed)
		}
		return nil
	},
}

func gitRepoRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("not a git repo: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func init() {
	worktreeCreateCmd.Flags().StringVar(&worktreeFromBranch, "from", "", "Base branch to create from (default: current HEAD)")
	worktreeCmd.AddCommand(worktreeCreateCmd)
	worktreeCmd.AddCommand(worktreeListCmd)
	worktreeCmd.AddCommand(worktreeCleanupCmd)
}
