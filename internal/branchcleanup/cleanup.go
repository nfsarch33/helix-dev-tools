package branchcleanup

import "strings"

// GitRunner abstracts git command execution for testability.
type GitRunner interface {
	Run(args ...string) (string, error)
}

// Result holds the outcome of a branch cleanup operation.
type Result struct {
	Repo          string
	LocalDeleted  []string
	RemoteDeleted []string
	Skipped       []string
	DryRun        bool
	Err           error
}

// Options controls which branch cleanup actions are performed.
type Options struct {
	DryRun             bool
	PruneStaleTracking bool
	DeleteLocalMerged  bool
	DeleteRemoteMerged bool
	DetectSquashMerged bool
}

// Repo identifies one repository in a fleet cleanup pass.
type Repo struct {
	Alias string
	Path  string
}

// FleetResult holds a cleanup result with the alias that produced it.
type FleetResult struct {
	Alias  string
	Result Result
}

// RunnerFactory creates a GitRunner for a repository path.
type RunnerFactory func(repoPath string) GitRunner

var protectedBranches = map[string]bool{
	"main":    true,
	"master":  true,
	"develop": true,
}

// CleanupFleet runs Cleanup across repositories.
func CleanupFleet(repos []Repo, factory RunnerFactory, dryRun bool) []FleetResult {
	return CleanupFleetWithOptions(repos, factory, Options{
		DryRun:             dryRun,
		DeleteLocalMerged:  true,
		DeleteRemoteMerged: true,
	})
}

// CleanupFleetWithOptions runs CleanupWithOptions across repositories.
func CleanupFleetWithOptions(repos []Repo, factory RunnerFactory, opts Options) []FleetResult {
	results := make([]FleetResult, 0, len(repos))
	for _, repo := range repos {
		runner := factory(repo.Path)
		results = append(results, FleetResult{
			Alias:  repo.Alias,
			Result: CleanupWithOptions(runner, repo.Path, opts),
		})
	}
	return results
}

// Cleanup removes local and remote branches that have been merged into main.
func Cleanup(runner GitRunner, repoPath string, dryRun bool) Result {
	return CleanupWithOptions(runner, repoPath, Options{
		DryRun:             dryRun,
		DeleteLocalMerged:  true,
		DeleteRemoteMerged: true,
	})
}

// CleanupWithOptions removes the requested merged branches and can refresh
// stale tracking refs before the branch scan.
func CleanupWithOptions(runner GitRunner, repoPath string, opts Options) Result {
	result := Result{Repo: repoPath, DryRun: opts.DryRun}

	if opts.PruneStaleTracking {
		if _, err := runner.Run("fetch", "--prune", "origin"); err != nil {
			result.Err = err
			return result
		}
	}

	currentOut, err := runner.Run("branch", "--show-current")
	if err != nil {
		result.Err = err
		return result
	}
	current := strings.TrimSpace(currentOut)

	if opts.DeleteLocalMerged {
		localOut, err := runner.Run("branch", "--merged", "main")
		if err != nil {
			result.Err = err
			return result
		}
		localBranches := parseBranches(localOut)

		for _, b := range localBranches {
			if protectedBranches[b] || b == current {
				result.Skipped = append(result.Skipped, b)
				continue
			}
			if opts.DryRun {
				result.LocalDeleted = append(result.LocalDeleted, b)
			} else {
				_, delErr := runner.Run("branch", "-d", b)
				if delErr != nil {
					result.Skipped = append(result.Skipped, b)
				} else {
					result.LocalDeleted = append(result.LocalDeleted, b)
				}
			}
		}
	}

	if opts.DetectSquashMerged {
		noMergedOut, err := runner.Run("branch", "--no-merged", "main")
		if err == nil {
			candidates := parseBranches(noMergedOut)
			for _, b := range candidates {
				if protectedBranches[b] || b == current {
					continue
				}
				cherryOut, cherryErr := runner.Run("log", "--oneline", "--cherry-pick", "--right-only", "main..."+b)
				if cherryErr != nil {
					continue
				}
				if strings.TrimSpace(cherryOut) == "" {
					if opts.DryRun {
						result.LocalDeleted = append(result.LocalDeleted, b)
					} else {
						_, delErr := runner.Run("branch", "-d", b)
						if delErr != nil {
							result.Skipped = append(result.Skipped, b)
						} else {
							result.LocalDeleted = append(result.LocalDeleted, b)
						}
					}
				}
			}
		}
	}

	if opts.DeleteRemoteMerged {
		remoteOut, err := runner.Run("branch", "-r", "--merged", "origin/main")
		if err != nil {
			result.Err = err
			return result
		}
		remoteBranches := parseRemoteBranches(remoteOut)

		for _, b := range remoteBranches {
			if opts.DryRun {
				result.RemoteDeleted = append(result.RemoteDeleted, b)
			} else {
				_, delErr := runner.Run("push", "origin", "--delete", b)
				if delErr != nil {
					result.Skipped = append(result.Skipped, "origin/"+b)
				} else {
					result.RemoteDeleted = append(result.RemoteDeleted, b)
				}
			}
		}
	}

	return result
}

func parseBranches(output string) []string {
	var branches []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "* ")
		line = strings.TrimPrefix(line, "+ ")
		if line == "" {
			continue
		}
		branches = append(branches, line)
	}
	return branches
}

func parseRemoteBranches(output string) []string {
	var branches []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, "->") {
			continue
		}
		if line == "origin/main" || line == "origin/master" {
			continue
		}
		name := strings.TrimPrefix(line, "origin/")
		branches = append(branches, name)
	}
	return branches
}
