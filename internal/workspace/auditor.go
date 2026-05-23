package workspace

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

type Runner interface {
	Run(ctx context.Context, dir string, args ...string) (string, error)
}

type RunnerFunc func(ctx context.Context, dir string, args ...string) (string, error)

func (f RunnerFunc) Run(ctx context.Context, dir string, args ...string) (string, error) {
	return f(ctx, dir, args...)
}

type GitRunner struct{}

func (GitRunner) Run(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

type Auditor struct {
	runner Runner
	now    func() time.Time
}

type AuditOptions struct {
	ConfigPath string
	Home       string
	RepoFilter []string
	Repos      []RepoConfig
	Quick      bool
	Timeout    time.Duration
}

func NewAuditor(runner Runner) *Auditor {
	if runner == nil {
		runner = GitRunner{}
	}
	return &Auditor{runner: runner, now: time.Now}
}

func (a *Auditor) Audit(ctx context.Context, opts AuditOptions) (AuditReport, error) {
	repos := opts.Repos
	if len(repos) == 0 {
		loaded, err := LoadRunxRepos(opts.ConfigPath, opts.Home, opts.RepoFilter)
		if err != nil {
			return AuditReport{}, err
		}
		repos = loaded
	}
	report := AuditReport{GeneratedAt: a.now().UTC(), Repos: make([]RepoStatus, 0, len(repos))}
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	for _, repo := range repos {
		repoCtx, cancel := context.WithTimeout(ctx, timeout)
		status := a.auditRepo(repoCtx, repo, opts.Quick)
		cancel()
		report.Repos = append(report.Repos, status)
	}
	return report, nil
}

func (a *Auditor) auditRepo(ctx context.Context, repo RepoConfig, quick bool) RepoStatus {
	status := RepoStatus{
		Alias:       repo.Alias,
		Path:        repo.Path,
		Default:     defaultBranch(repo.DefaultBranch),
		GeneratedAt: a.now().UTC(),
	}
	if !a.auditBranch(ctx, repo, &status) {
		return status
	}
	a.auditDirty(ctx, repo, &status)
	a.auditTracking(ctx, repo, &status)
	a.auditStaleTracking(ctx, repo, &status)
	if !quick {
		a.auditMainRef(ctx, repo, &status)
	}
	return status
}

func (a *Auditor) auditBranch(ctx context.Context, repo RepoConfig, status *RepoStatus) bool {
	branch, err := a.git(ctx, repo.Path, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		addFinding(status, FindingAuditError, SeverityWarning, "git branch audit failed")
		return false
	}
	status.Branch = strings.TrimSpace(branch)
	if status.Branch == "HEAD" {
		addFinding(status, FindingDetachedHead, SeverityWarning, "repository is in detached HEAD state")
	}
	return true
}

func (a *Auditor) auditDirty(ctx context.Context, repo RepoConfig, status *RepoStatus) {
	porcelain, err := a.git(ctx, repo.Path, "status", "--porcelain")
	if err == nil && strings.TrimSpace(porcelain) != "" {
		// v322-5: race-protected repos (vendor mirrors + hands-off
		// EC-agent territory) get the FindingDirtyRaceProtected code
		// so the scorer applies the lower weight (8 vs 25). The
		// underlying remediation prompt (commit/stash) is the same;
		// the score just reflects that the worker can't actually
		// remediate findings outside its authority.
		if repo.VendorMirror || repo.RaceProtected {
			addFinding(status, FindingDirtyRaceProtected, SeverityWarning,
				"modified or untracked files in race-protected repo (vendor mirror or hands-off agent territory)")
			return
		}
		addFinding(status, FindingDirtyWorktree, SeverityHard, "modified or untracked files are present")
	}
}

func (a *Auditor) auditTracking(ctx context.Context, repo RepoConfig, status *RepoStatus) {
	status.Ahead = a.countGit(ctx, repo.Path, "rev-list", "--count", "@{u}..HEAD")
	status.Behind = a.countGit(ctx, repo.Path, "rev-list", "--count", "HEAD..@{u}")
	if status.Ahead > 0 {
		addFinding(status, FindingUnpushedCommits, SeverityWarning, fmt.Sprintf("%d commit(s) ahead of upstream", status.Ahead))
	}
	if status.Behind > 0 {
		addFinding(status, behindCode(repo), SeverityInfo, fmt.Sprintf("%d commit(s) behind upstream", status.Behind))
	}
}

func (a *Auditor) auditStaleTracking(ctx context.Context, repo RepoConfig, status *RepoStatus) {
	branchVV, err := a.git(ctx, repo.Path, "branch", "-vv")
	if err == nil && strings.Contains(branchVV, ": gone]") {
		addFinding(status, FindingStaleTrackingRef, SeverityInfo, "local branch tracks a deleted remote branch")
	}
}

// auditMainRef ensures the repository has a local copy of its
// configured default branch (typically `main`, but `master` for
// mission-control and other legacy repos). Pre-fix the check
// hard-coded "main", which falsely flagged repos that had explicitly
// declared `default_branch: master` in the runx config. The repo
// path's `.git` existence guard stays so non-git fixtures don't
// trip the finding.
func (a *Auditor) auditMainRef(ctx context.Context, repo RepoConfig, status *RepoStatus) {
	branch := strings.TrimSpace(status.Default)
	if branch == "" {
		return
	}
	ref := "refs/heads/" + branch
	if _, err := a.git(ctx, repo.Path, "show-ref", "--verify", "--quiet", ref); err != nil {
		if _, statErr := os.Stat(filepath.Join(repo.Path, ".git")); statErr == nil {
			addFinding(status, FindingNoMainRef, SeverityWarning, "repository has no local "+branch+" branch")
		}
	}
}

func addFinding(status *RepoStatus, code FindingCode, severity Severity, message string) {
	status.Findings = append(status.Findings, Finding{
		Code:     code,
		Severity: severity,
		Message:  message,
	})
}

func behindCode(repo RepoConfig) FindingCode {
	if repo.VendorMirror {
		return FindingVendorBehind
	}
	return FindingBehindDefault
}

func (a *Auditor) git(ctx context.Context, dir string, args ...string) (string, error) {
	return a.runner.Run(ctx, dir, args...)
}

func (a *Auditor) countGit(ctx context.Context, dir string, args ...string) int {
	out, err := a.git(ctx, dir, args...)
	if err != nil {
		return 0
	}
	value, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0
	}
	return value
}

func LoadRunxRepos(configPath, home string, filters []string) ([]RepoConfig, error) {
	if home == "" {
		home = os.Getenv("HOME")
	}
	if configPath == "" {
		configPath = filepath.Join(home, ".config", "runx", "config.yaml")
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read runx config: %w", err)
	}
	var cfg struct {
		Repos map[string]struct {
			Path          string `yaml:"path"`
			Identity      string `yaml:"identity"`
			DefaultBranch string `yaml:"default_branch"`
			VendorMirror  bool   `yaml:"vendor_mirror"`
		} `yaml:"repos"`
	}
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse runx config: %w", err)
	}
	filterSet := make(map[string]bool, len(filters))
	for _, filter := range filters {
		filterSet[filter] = true
	}
	aliases := make([]string, 0, len(cfg.Repos))
	for alias := range cfg.Repos {
		if len(filterSet) > 0 && !filterSet[alias] {
			continue
		}
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)
	repos := make([]RepoConfig, 0, len(aliases))
	for _, alias := range aliases {
		repo := cfg.Repos[alias]
		repos = append(repos, RepoConfig{
			Alias:         alias,
			Path:          expandHome(repo.Path, home),
			Identity:      repo.Identity,
			DefaultBranch: defaultBranch(repo.DefaultBranch),
			VendorMirror:  repo.VendorMirror || isKnownVendorMirror(alias),
			RaceProtected: isKnownRaceProtectedAlias(alias),
		})
	}
	return repos, nil
}

func defaultBranch(value string) string {
	if strings.TrimSpace(value) == "" {
		return "main"
	}
	return strings.TrimSpace(value)
}

func expandHome(path, home string) string {
	path = strings.ReplaceAll(path, "$HOME", home)
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	return path
}

// isKnownVendorMirror reports whether the runx alias is a vendor
// mirror that should be classified as `vendor_behind` (info severity)
// rather than `behind_default` (warning) when its main lags upstream.
//
// The canonical list is mirrored from
// `global-memories/daily-startup-prompt.md` and
// `global-memories/repo-index.md`. Vendor mirrors are analysis-only
// checkouts of upstream projects we read from but do not own.
// Out-of-date `main` is the expected steady state for them and must
// not block Workspace Doctor scoring.
//
// Members:
//   - helixon, openclaw, hermes, gstack, temporal: analysis-only
//     forks tracked in daily-startup-prompt.md
//   - windows-mcp: "Reference checkout" per repo-index.md (community
//     OSS upstream we use but do not maintain)
func isKnownVendorMirror(alias string) bool {
	switch alias {
	case "helixon", "openclaw", "hermes", "gstack", "temporal", "windows-mcp":
		return true
	default:
		return false
	}
}

// isKnownRaceProtectedAlias reports whether the runx alias is owned
// by a hands-off agent (EC-agent territory) or operator (resume,
// global-kb stash@{0}) and therefore should not be REDed by the
// workspace doctor when dirty. The list is mirrored from the
// "Race Boundaries" section of recent sprint plans (v320 onwards).
// Vendor mirrors get the same treatment via isKnownVendorMirror;
// this list is for non-vendor-mirror aliases that nonetheless lie
// outside the worker's remediation authority.
//
// Members:
//   - business, ai-agent-business-stack, ecommerce, agentic-ecommerce-web:
//     EC-agent territory (hands-off; declared in race boundaries)
//   - resume: operator territory (job-hunt cashflow ledger)
//
// Sentinel: this list mirrors race-boundary declarations from
// roadmap-v322-v331.md hard rules and v320-onward sprint plans.
// Update both together.
func isKnownRaceProtectedAlias(alias string) bool {
	switch alias {
	case "business", "ecommerce", "agentic-ecommerce-web", "resume":
		return true
	default:
		return false
	}
}
