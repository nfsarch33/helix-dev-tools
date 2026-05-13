package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/cursor-tools/internal/clilog"
	"github.com/nfsarch33/cursor-tools/internal/config"
	"github.com/nfsarch33/cursor-tools/internal/coordination"
)

var sessionHandoffForce bool
var sessionHandoffDryRun bool

var sessionHandoffCmd = &cobra.Command{
	Use:   "session-handoff",
	Short: "Write a session handoff document to global-memories",
	Long: `Writes ~/memo/global-memories/session-handoff-<YYYY-MM-DD>-<platform>.md with
current platform, git state for all tracked repos, and open-items stubs.
Skips if today's platform-specific file already exists unless --force is given.`,
	RunE: runSessionHandoff,
}

var handoffReviewCmd = &cobra.Command{
	Use:   "handoff-review",
	Short: "Fetch and display session handoffs from other machines before pulling",
	Long: `Fetches origin without merging, finds new/changed session-handoff-*.md files
on the remote, displays their content, and records the check in
~/.cursor/hooks/handoff-last-check.txt. Run this before git pull to be aware
of what other machines have been doing.`,
	RunE: runHandoffReview,
}

func init() {
	sessionHandoffCmd.Flags().BoolVar(&sessionHandoffForce, "force", false, "Overwrite today's handoff file if it already exists")
	sessionHandoffCmd.Flags().BoolVar(&sessionHandoffDryRun, "dry-run", false, "Show what would be written without creating the file")
}

func runSessionHandoff(_ *cobra.Command, _ []string) error {
	p := config.DefaultPaths()
	out := clilog.NewPrefixed("[session-handoff]")
	return generateSessionHandoff(p, out, sessionHandoffForce, sessionHandoffDryRun)
}

// platformSuffix returns a short label for the current machine: "macos", "wsl", or "linux".
func platformSuffix() string {
	switch {
	case runtime.GOOS == "darwin":
		return "macos"
	case os.Getenv("WSL_INTEROP") != "" || os.Getenv("WSL_DISTRO_NAME") != "":
		return "wsl"
	default:
		return runtime.GOOS
	}
}

// generateSessionHandoff is the testable core of the command.
func generateSessionHandoff(p config.Paths, out *clilog.Prefixed, force, dryRun bool) error {
	today := time.Now().UTC().Format("2006-01-02")
	suffix := platformSuffix()
	outPath := filepath.Join(p.GlobalMemoriesDir(), "session-handoff-"+today+"-"+suffix+".md")

	if !force {
		if _, err := os.Stat(outPath); err == nil {
			out.Info("handoff for %s already exists (use --force to overwrite): %s", today, outPath)
			return nil
		}
	}

	content := buildHandoffContent(p, today)

	if dryRun {
		out.Info("[dry-run] would write: %s", outPath)
		out.Info("--- preview ---\n%s", content)
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	if err := os.WriteFile(outPath, []byte(content), 0o644); err != nil { // #nosec G306 -- personal config file
		return fmt.Errorf("write handoff: %w", err)
	}
	out.Info("written: %s", outPath)
	return nil
}

// fetchHandoffSignals is replaceable for testing. Returns coordination signals
// for the current machine, or nil if Mem0 is unreachable.
var fetchHandoffSignals = defaultFetchHandoffSignals

func defaultFetchHandoffSignals(p config.Paths, machine string) []coordination.Signal {
	client, err := newCoordinationClient(p)
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	signals, err := client.ListSignals(ctx)
	if err != nil {
		return nil
	}
	return coordination.FilterForMachine(signals, machine)
}

func buildHandoffContent(p config.Paths, today string) string {
	hostname, _ := os.Hostname()
	platform := p.PlatformProfile()
	signals := fetchHandoffSignals(p, platform)
	runtime := collectHandoffRuntimeContext(p)

	return renderHandoff(hostname, platform, today, buildRepoSection(p), signals, runtime)
}

type handoffRuntimeContext struct {
	Workspace handoffWorkspaceContext
	Resource  resourceProbeSnapshot
}

type handoffWorkspaceContext struct {
	Path      string
	GitRoot   string
	Branch    string
	RepoAlias string
	Mode      string
}

func collectHandoffRuntimeContext(p config.Paths) handoffRuntimeContext {
	return handoffRuntimeContext{
		Workspace: detectHandoffWorkspace(p),
		Resource:  readLastProbeEntry(resourceProbePath()),
	}
}

func detectHandoffWorkspace(p config.Paths) handoffWorkspaceContext {
	workspace := strings.TrimSpace(os.Getenv("CURSOR_WORKSPACE"))
	if workspace == "" || !filepath.IsAbs(workspace) {
		if wd, err := os.Getwd(); err == nil && filepath.IsAbs(wd) {
			workspace = wd
		}
	}
	if workspace == "" {
		return handoffWorkspaceContext{}
	}

	workspace = filepath.Clean(workspace)
	ctx := handoffWorkspaceContext{Path: workspace}

	if repoAlias := parseRunxWorktreeAlias(p.Home, workspace); repoAlias != "" {
		ctx.Mode = "runx-managed worktree"
		ctx.RepoAlias = repoAlias
	}

	if gitRoot := gitOutputStr(workspace, "rev-parse", "--show-toplevel"); gitRoot != "" && gitRoot != "(unavailable)" {
		ctx.GitRoot = gitRoot
	}
	if branch := gitOutputStr(workspace, "branch", "--show-current"); branch != "" && branch != "(unavailable)" {
		ctx.Branch = branch
	}
	if ctx.Mode == "" {
		switch {
		case ctx.GitRoot != "":
			ctx.Mode = "git workspace"
		default:
			ctx.Mode = "non-git workspace"
		}
	}
	if ctx.RepoAlias == "" {
		ctx.RepoAlias = inferWorkspaceRepoAlias(p, ctx.GitRoot, workspace)
	}

	return ctx
}

func parseRunxWorktreeAlias(home, workspace string) string {
	base := filepath.Join(home, "runs", "worktrees") + string(os.PathSeparator)
	if !strings.HasPrefix(workspace, base) {
		return ""
	}
	rel := strings.TrimPrefix(workspace, base)
	parts := strings.Split(rel, string(os.PathSeparator))
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return ""
	}
	return parts[0]
}

func inferWorkspaceRepoAlias(p config.Paths, gitRoot, workspace string) string {
	candidate := strings.TrimSpace(gitRoot)
	if candidate == "" {
		candidate = workspace
	}
	switch candidate {
	case p.GlobalKB:
		return "global-kb"
	}
	return filepath.Base(candidate)
}

// renderHandoff is the pure, testable function.
func renderHandoff(hostname, platform, today, repoSection string, signals []coordination.Signal, runtime handoffRuntimeContext) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Session Handoff: %s\n\n", today))
	sb.WriteString(fmt.Sprintf("**Date**: %s  \n", today))
	sb.WriteString(fmt.Sprintf("**Machine**: %s (%s)  \n", hostname, platform))
	sb.WriteString(fmt.Sprintf("**Generated**: %s UTC\n\n", time.Now().UTC().Format("2006-01-02 15:04:05")))
	sb.WriteString("---\n\n")

	signalContent := coordination.RenderHandoffSection(signals)

	if signalContent != "" {
		sb.WriteString(signalContent)
	} else {
		sb.WriteString("## Task Summary\n\n")
		sb.WriteString("No cross-machine signals were available when this handoff was generated. Review the current repository state and latest durable evidence before resuming work.\n\n")
		sb.WriteString("## Decisions Made\n\n")
		sb.WriteString("- No new cross-machine decisions were detected by the signal reader.\n\n")
	}

	sb.WriteString(renderWorkspaceSection(runtime.Workspace))
	sb.WriteString(renderResourceSection(runtime.Resource))

	sb.WriteString("## Current State\n\n")
	sb.WriteString(repoSection)

	sb.WriteString("## Open Items\n\n")
	if hasBlockerSignals(signals) {
		for _, s := range signals {
			if s.Type == coordination.SignalBlocker {
				sb.WriteString(fmt.Sprintf("- **Blocker**: %s\n", s.Message))
			}
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString("- Review current branch status and the latest sprint evidence before starting new work.\n\n")
	}

	sb.WriteString("## Resume Instructions\n\n")
	sb.WriteString("1. Run: `runx doctor && runx config check-aliases v302 && runx history audit && runx workspace doctor --quick`\n")
	sb.WriteString("2. Run: `runx cursor-tools handoff-review` and `runx cursor-tools signal list`\n")
	if runtime.Workspace.Mode == "runx-managed worktree" && runtime.Workspace.RepoAlias != "" {
		sb.WriteString(fmt.Sprintf("3. Verify the active worktree: `runx worktree list --repo %s`\n", runtime.Workspace.RepoAlias))
		sb.WriteString(fmt.Sprintf("4. Resume inside the `%s` worktree; avoid the canonical checkout unless ownership is explicitly transferred.\n", runtime.Workspace.RepoAlias))
		sb.WriteString("5. Use `runx env personal-shell` or `runx env scrub --` before any mutating repo work.\n")
		sb.WriteString("6. Review the active sprint roadmap and latest durable evidence before making changes.\n\n")
	} else {
		sb.WriteString("3. Use `runx env personal-shell` or `runx env scrub --` before any mutating repo work.\n")
		sb.WriteString("4. Review the active sprint roadmap and latest durable evidence before making changes.\n\n")
	}

	sb.WriteString("## Context Files\n\n")
	sb.WriteString("- `global-memories/daily-startup-prompt.md`\n")
	sb.WriteString("- `global-memories/session-kickoff-template.md`\n")
	sb.WriteString("- latest `reports/research/post-*-qa-*.md` evidence file\n\n")

	sb.WriteString("---\n\n")
	sb.WriteString("*Generated by `cursor-tools session-handoff`.*\n")

	return sb.String()
}

func renderWorkspaceSection(workspace handoffWorkspaceContext) string {
	var sb strings.Builder
	sb.WriteString("## Active Workspace\n\n")
	if workspace.Path == "" {
		sb.WriteString("- No active workspace detected.\n\n")
		return sb.String()
	}
	sb.WriteString(fmt.Sprintf("- Workspace: `%s`\n", workspace.Path))
	sb.WriteString(fmt.Sprintf("- Mode: `%s`\n", workspace.Mode))
	if workspace.RepoAlias != "" {
		sb.WriteString(fmt.Sprintf("- Repo alias: `%s`\n", workspace.RepoAlias))
	}
	if workspace.GitRoot != "" {
		sb.WriteString(fmt.Sprintf("- Git root: `%s`\n", workspace.GitRoot))
	}
	if workspace.Branch != "" {
		sb.WriteString(fmt.Sprintf("- Branch: `%s`\n", workspace.Branch))
	}
	sb.WriteString("\n")
	return sb.String()
}

func renderResourceSection(probe resourceProbeSnapshot) string {
	var sb strings.Builder
	sb.WriteString("## Resource Snapshot\n\n")
	probe = normalizeResourceProbeSnapshot(probe)
	if probe.Tier == "UNKNOWN" && probe.Err != "" && probe.FreePct == 0 {
		sb.WriteString(fmt.Sprintf("- Resource probe unavailable: `%s`\n\n", probe.Err))
		return sb.String()
	}
	sb.WriteString(fmt.Sprintf("- Tier: `%s`\n", probe.Tier))
	if probe.FreePct >= 0 {
		sb.WriteString(fmt.Sprintf("- Free memory: `%d%%`\n", probe.FreePct))
	}
	sb.WriteString(fmt.Sprintf("- Sentrux desktop processes: `%d`\n", probe.SentruxDesktopProcesses))
	sb.WriteString(fmt.Sprintf("- Sentrux MCP processes: `%d`\n", probe.SentruxMCPProcesses))
	if probe.Ts != "" {
		sb.WriteString(fmt.Sprintf("- Sample time: `%s`\n", probe.Ts))
	}
	if probe.Err != "" {
		sb.WriteString(fmt.Sprintf("- Note: `%s`\n", probe.Err))
	}
	sb.WriteString("\n")
	return sb.String()
}

func hasBlockerSignals(signals []coordination.Signal) bool {
	for _, s := range signals {
		if s.Type == coordination.SignalBlocker {
			return true
		}
	}
	return false
}

// buildRepoSection collects git state for GlobalKB and repos listed in repos-to-sync.txt.
func buildRepoSection(p config.Paths) string {
	var sb strings.Builder

	repos := []string{p.GlobalKB}
	listFile := filepath.Join(p.ToolsDir(), "repos-to-sync.txt")
	if f, err := os.Open(listFile); err == nil {
		defer f.Close()
		lines := make([]byte, 0, 4096)
		buf := make([]byte, 4096)
		for {
			n, err := f.Read(buf)
			lines = append(lines, buf[:n]...)
			if err != nil {
				break
			}
		}
		for _, line := range strings.Split(string(lines), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if isDir(line) {
				repos = append(repos, line)
			}
		}
	}

	for _, repo := range repos {
		name := filepath.Base(repo)
		branch := gitOutputStr(repo, "rev-parse", "--abbrev-ref", "HEAD")
		commit := gitOutputStr(repo, "log", "--oneline", "-1")
		status := gitOutputStr(repo, "status", "--short")
		dirty := "clean"
		if strings.TrimSpace(status) != "" {
			lines := strings.Count(strings.TrimSpace(status), "\n") + 1
			dirty = fmt.Sprintf("%d uncommitted file(s)", lines)
		}
		sb.WriteString(fmt.Sprintf("### %s\n\n", name))
		sb.WriteString(fmt.Sprintf("- Branch: `%s`\n", branch))
		sb.WriteString(fmt.Sprintf("- Last commit: `%s`\n", commit))
		sb.WriteString(fmt.Sprintf("- Working tree: %s\n\n", dirty))
	}
	return sb.String()
}

func gitOutputStr(repoPath string, args ...string) string {
	fullArgs := append([]string{"-C", repoPath}, args...)
	out, err := exec.Command("git", fullArgs...).Output()
	if err != nil {
		return "(unavailable)"
	}
	return strings.TrimSpace(string(out))
}

// HandoffCheckStateFile returns the path of the state file that records the last
// successful handoff-review run.
func HandoffCheckStateFile(p config.Paths) string {
	return filepath.Join(p.HooksDir, "handoff-last-check.txt")
}

func runHandoffReview(_ *cobra.Command, _ []string) error {
	p := config.DefaultPaths()
	out := clilog.NewPrefixed("[handoff-review]")
	found, err := previewRemoteHandoffs(p, out)
	if err != nil {
		return err
	}
	if !found {
		out.Info("no new handoffs from other machines (remote is up-to-date or offline)")
	}
	return nil
}

// previewRemoteHandoffs is the testable core of the handoff-review command.
// It fetches origin, finds new/changed session-handoff-*.md files on the remote
// relative to the local HEAD, displays them, and records the check in the state file.
// Returns true if at least one remote handoff was found and displayed.
func previewRemoteHandoffs(p config.Paths, out *clilog.Prefixed) (bool, error) {
	repoPath := p.GlobalKB

	// Fetch without merging so we can inspect what's coming.
	if err := gitCmd(repoPath, "fetch", "origin", "--quiet"); err != nil {
		out.Warn("fetch failed (offline?): %v", err)
		return false, nil
	}

	// Find files that exist on origin/main but differ from HEAD.
	diffOut, err := exec.Command("git", "-C", repoPath,
		"diff", "HEAD..origin/main", "--name-only").Output()
	if err != nil {
		// No commits ahead — nothing to review.
		return false, nil
	}

	var handoffFiles []string
	for _, line := range strings.Split(strings.TrimSpace(string(diffOut)), "\n") {
		line = strings.TrimSpace(line)
		// Match global-memories/session-handoff-*.md
		if strings.HasPrefix(line, "global-memories/session-handoff-") &&
			strings.HasSuffix(line, ".md") {
			handoffFiles = append(handoffFiles, line)
		}
	}

	checkedAt := time.Now().UTC().Format(time.RFC3339)
	if err := recordHandoffCheck(p, checkedAt, handoffFiles); err != nil {
		out.Warn("could not write state file: %v", err)
	}

	if len(handoffFiles) == 0 {
		return false, nil
	}

	out.Info("=== %d remote handoff(s) from other machines ===", len(handoffFiles))
	for _, f := range handoffFiles {
		out.Info("--- %s ---", f)
		content, showErr := exec.Command("git", "-C", repoPath,
			"show", "origin/main:"+f).Output()
		if showErr != nil {
			out.Warn("could not read %s: %v", f, showErr)
			continue
		}
		// Print only the first 60 lines to keep output manageable.
		lines := strings.Split(string(content), "\n")
		limit := 60
		if len(lines) < limit {
			limit = len(lines)
		}
		fmt.Println(strings.Join(lines[:limit], "\n"))
		if len(lines) > 60 {
			out.Info("... (%d more lines — read %s after pull for full content)", len(lines)-60, f)
		}
	}
	out.Info("=== end of remote handoffs ===")
	return true, nil
}

// recordHandoffCheck writes the state file so suiteHandoffAcknowledgement can verify
// the review ran today.
func recordHandoffCheck(p config.Paths, checkedAt string, files []string) error {
	stateFile := HandoffCheckStateFile(p)
	if err := os.MkdirAll(filepath.Dir(stateFile), 0o755); err != nil {
		return err
	}
	var sb strings.Builder
	sb.WriteString("checked: " + checkedAt + "\n")
	if len(files) > 0 {
		sb.WriteString("files:\n")
		for _, f := range files {
			sb.WriteString("  " + f + "\n")
		}
	} else {
		sb.WriteString("files: none\n")
	}
	return os.WriteFile(stateFile, []byte(sb.String()), 0o644) // #nosec G306 -- personal state file
}
