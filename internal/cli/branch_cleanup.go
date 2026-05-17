package cli

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/helix-dev-tools/internal/branchcleanup"
	"github.com/nfsarch33/helix-dev-tools/internal/workspace"
)

type execGitRunner struct {
	repoPath string
}

func (r *execGitRunner) Run(args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", r.repoPath}, args...)...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

var branchCleanupDryRun bool
var branchCleanupForce bool
var branchCleanupFleet bool
var branchCleanupSquash bool
var branchCleanupRepos []string
var branchCleanupConfig string

var branchCleanupCmd = &cobra.Command{
	Use:   "branch-cleanup",
	Short: "Delete local and remote branches already merged into main",
	Long:  "Scans for branches merged into main/origin/main and deletes them. Defaults to --dry-run; pass --force to actually delete. Use --squash to also detect squash-merged branches.",
	RunE: func(_ *cobra.Command, _ []string) error {
		dryRun := !branchCleanupForce
		if branchCleanupDryRun {
			dryRun = true
		}

		opts := branchcleanup.Options{
			DryRun:             dryRun,
			DeleteLocalMerged:  true,
			DeleteRemoteMerged: true,
			DetectSquashMerged: branchCleanupSquash,
		}

		if branchCleanupFleet {
			repos, err := loadBranchCleanupRepos()
			if err != nil {
				return err
			}
			results := branchcleanup.CleanupFleetWithOptions(repos, func(path string) branchcleanup.GitRunner {
				return &execGitRunner{repoPath: path}
			}, opts)
			for _, result := range results {
				fmt.Printf("\n[%s]\n", result.Alias)
				if result.Result.Err != nil {
					fmt.Printf("  ERROR: %v\n", result.Result.Err)
					continue
				}
				printBranchCleanupResult(result.Result)
			}
			return nil
		}

		runner := &execGitRunner{repoPath: "."}
		result := branchcleanup.CleanupWithOptions(runner, ".", opts)
		if result.Err != nil {
			return fmt.Errorf("branch-cleanup: %w", result.Err)
		}
		printBranchCleanupResult(result)
		return nil
	},
}

func init() {
	branchCleanupCmd.Flags().BoolVar(&branchCleanupDryRun, "dry-run", false, "Show what would be deleted without acting (default behavior)")
	branchCleanupCmd.Flags().BoolVar(&branchCleanupForce, "force", false, "Actually delete merged branches")
	branchCleanupCmd.Flags().BoolVar(&branchCleanupFleet, "fleet", false, "Run across runx-configured repos")
	branchCleanupCmd.Flags().BoolVar(&branchCleanupSquash, "squash", false, "Also detect squash-merged branches via cherry-pick diff")
	branchCleanupCmd.Flags().StringSliceVar(&branchCleanupRepos, "repo-alias", nil, "Runx repo alias to include; repeatable")
	branchCleanupCmd.Flags().StringVar(&branchCleanupConfig, "config", "", "runx config path")
}

func loadBranchCleanupRepos() ([]branchcleanup.Repo, error) {
	loaded, err := workspace.LoadRunxRepos(branchCleanupConfig, "", branchCleanupRepos)
	if err != nil {
		return nil, fmt.Errorf("branch-cleanup fleet: %w", err)
	}
	repos := make([]branchcleanup.Repo, 0, len(loaded))
	for _, repo := range loaded {
		repos = append(repos, branchcleanup.Repo{Alias: repo.Alias, Path: repo.Path})
	}
	return repos, nil
}

func printBranchCleanupResult(result branchcleanup.Result) {
	if result.DryRun {
		fmt.Println("[dry-run] Would delete the following branches:")
	} else {
		fmt.Println("Deleted branches:")
	}
	if len(result.LocalDeleted) > 0 {
		fmt.Println("  Local:")
		for _, b := range result.LocalDeleted {
			fmt.Printf("    %s\n", b)
		}
	}
	if len(result.RemoteDeleted) > 0 {
		fmt.Println("  Remote:")
		for _, b := range result.RemoteDeleted {
			fmt.Printf("    origin/%s\n", b)
		}
	}
	if len(result.Skipped) > 0 {
		fmt.Printf("  Skipped: %s\n", strings.Join(result.Skipped, ", "))
	}
	if len(result.LocalDeleted) == 0 && len(result.RemoteDeleted) == 0 {
		fmt.Println("  (none)")
	}
}
