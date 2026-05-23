// runx-public-repo-gate: allow-file personal_path_id — identity gate detects literal personal-stack identifiers, so the strings must remain in source

// Package cli — git pre-push handler.
//
// This hook layers four policies:
//
//  1. Direct push to main/master is blocked unless the worktree opts in
//     with `git config hooks.allowMainPush true` (legacy behaviour).
//
//  2. On personal repos (nfsarch33/* remotes) the strict identity gate
//     runs and aborts the push if any poisoned GITHUB_TOKEN-style env
//     vars are present, the user.email is empty, or the email is not
//     the personal nfsarch33 identity. The Zendesk work clones are
//     never gated.
//
//  3. On personal repos that are PUBLIC on GitHub (per
//     `cursor-config/rules/public-repo-sanitization.mdc`), the
//     `runx public-repo-gate --repo <alias>` scan must pass. Any
//     blocking finding (Tailscale IP, fleet alias, secret env name,
//     etc.) aborts the push. Private repos and Zendesk work clones
//     skip this layer. New in v317-1.
//
//  4. On GitHub zendesk/* remotes (work clones), any non-fast-forward
//     ref update is blocked by inspecting pre-push stdin lines and
//     running `git merge-base --is-ancestor <remote-sha> <local-sha>`.
//     New-branch pushes (remote all-zero object name) are allowed;
//     ref deletions are not treated as force-push here. Operator
//     break-glass: git push --no-verify (same escape hatch as other
//     hook layers).
//
// Policy 2 is the v257 W2 sprint deliverable. Policy 3 is the v317-1
// sprint deliverable. Policy 4 complements org-wide Zendesk PR workflow.
// Policies 2–3 are documented in `sop/personal-repo-identity.md` and the
// public-repo sanitisation rule.
package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var prePushExit = os.Exit
var prePushStderr io.Writer = os.Stderr
var prePushStdin io.Reader

var (
	identityGateEvaluator   = evaluateIdentityGateStrict
	identityGateGatherer    = gatherIdentityGateState
	allowMainPushGetter     = readAllowMainPush
	publicRepoGateEvaluator = evaluatePublicRepoGateDefault
	publicRepoNameFromURLFn = publicRepoNameFromURLDefault
	isAncestorChecker       = gitIsAncestor
	rebrandGateEvaluator    = evaluateRebrandGateDefault
	sentruxGateEvaluator    = evaluateSentruxGateDefault
	sentruxGateEnabledGetter = readSentruxGateEnabled
)

// isZendeskGitHubRemote reports whether the push URL targets the
// github.com/zendesk org (SSH scp-style or https). Matches the work
// clone convention used across cursor-tools identity tests.
func isZendeskGitHubRemote(remoteURL string) bool {
	r := strings.ToLower(strings.TrimSpace(remoteURL))
	return strings.Contains(r, "github.com:zendesk/") ||
		strings.Contains(r, "github.com/zendesk/")
}

func isAllZeroGitObjectName(name string) bool {
	s := strings.TrimSpace(strings.ToLower(name))
	if s == "" {
		return false
	}
	for _, c := range s {
		if c != '0' {
			return false
		}
	}
	return true
}

func gitIsAncestor(remoteSHA, localSHA string) bool {
	if remoteSHA == "" || localSHA == "" {
		return false
	}
	cmd := exec.Command("git", "merge-base", "--is-ancestor", remoteSHA, localSHA)
	return cmd.Run() == nil
}

func isFullHexObjectName(name string) bool {
	s := strings.TrimSpace(strings.ToLower(name))
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > 'f' || (c > '9' && c < 'a') {
			return false
		}
	}
	return true
}

func zendeskNonFastForwardFailures(remoteURL string, stdinLines []string) []string {
	if !isZendeskGitHubRemote(remoteURL) {
		return nil
	}
	var out []string
	for _, line := range stdinLines {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 4 {
			continue
		}
		localRef, localSHA, remoteRef, remoteSHA := fields[0], fields[1], fields[2], fields[3]
		if localRef == "(delete)" {
			continue
		}
		if isAllZeroGitObjectName(remoteSHA) {
			continue
		}
		if !isFullHexObjectName(localSHA) || !isFullHexObjectName(remoteSHA) {
			continue
		}
		if isAncestorChecker(remoteSHA, localSHA) {
			continue
		}
		out = append(out, fmt.Sprintf(
			"non-fast-forward refused for Zendesk remote: %s -> %s (remote %s.. local %s..); use PR workflow instead of force or history rewrite",
			localRef, remoteRef, shortenSHA(remoteSHA), shortenSHA(localSHA),
		))
	}
	return out
}

func shortenSHA(hex string) string {
	s := strings.TrimSpace(hex)
	if len(s) <= 12 {
		return s
	}
	return s[:12]
}

// publicRepoGitHubNames is the closed enumeration of GitHub repo
// names (under nfsarch33/*) that are PUBLIC. Source of truth lives in
// `cursor-config/rules/public-repo-sanitization.mdc` "Public Repos
// (scope of this rule)" and the parallel runx
// `internal/cli/publicrepogate.go::publicRepoAliases`. This list uses
// the GitHub repo NAMES (not the runx aliases) because the pre-push
// hook only sees the URL, not the runx config. A future
// `runx config alias-of <github-name>` subcommand could collapse the
// duplication; for now both lists must move together.
var publicRepoGitHubNames = map[string]struct{}{
	"agentic-ecommerce-web": {},
	"ansible-win":           {},
	// Legacy name preserved for dual-remote period (origin still points here).
	"cursor-tools":    {},
	"fleet-bench":     {},
	"helix-dev-tools": {}, // Helixon R0 rename of cursor-tools
	// Legacy names preserved for dual-remote period.
	"helixon-mcp":          {},
	"helixon-ops":          {},
	"llm-cluster-router":    {},
	"mem0-mcp-go":           {},
	"minimax-openai-bridge": {},
	"offload-telemetry":     {},
	"pdf-mcp":               {},
	"seek-mcp":              {},
	"uiauto-framework":      {},
	"upwork-mcp":            {},
}

// publicRepoNameFromURLDefault extracts the GitHub repo name from a
// remote URL of the shape `git@<host>:nfsarch33/<repo>.git` or
// `https://github.com/nfsarch33/<repo>.git`. Returns "" when the URL
// is not a personal-stack remote or the parser cannot recover the
// repo segment.
func publicRepoNameFromURLDefault(remoteURL string) string {
	if remoteURL == "" {
		return ""
	}
	// Match against the personal owner segment so Zendesk work clones
	// never resolve to a public-gate alias.
	owner := "nfsarch33/"
	idx := strings.Index(remoteURL, owner)
	if idx == -1 {
		return ""
	}
	rest := remoteURL[idx+len(owner):]
	rest = strings.TrimSuffix(rest, ".git")
	rest = strings.TrimSuffix(rest, "/")
	if i := strings.IndexAny(rest, "/?"); i >= 0 {
		rest = rest[:i]
	}
	return rest
}

// evaluatePublicRepoGateDefault is the production implementation that
// shells out to `runx public-repo-gate --repo <alias>` against the
// repo's working tree. The runx alias matches the GitHub repo name
// for every entry in publicRepoGitHubNames except `llm-cluster-router`
// which is aliased as `router` in `~/.config/runx/config.yaml`. The
// translation lives in resolvePublicRepoAlias.
func evaluatePublicRepoGateDefault(remoteURL string) []string {
	repoName := publicRepoNameFromURLFn(remoteURL)
	if repoName == "" {
		return nil
	}
	if _, isPublic := publicRepoGitHubNames[repoName]; !isPublic {
		return nil
	}
	alias := resolvePublicRepoAlias(repoName)
	cmd := exec.Command("runx", "public-repo-gate", "--repo", alias)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	// The gate uses non-zero exit on findings; surface stderr+stdout
	// joined so the operator sees the structured findings table.
	body := strings.TrimSpace(string(out))
	if body == "" {
		body = err.Error()
	}
	return []string{
		fmt.Sprintf("public-repo-gate failed for nfsarch33/%s (alias=%s):", repoName, alias),
		body,
	}
}

// resolvePublicRepoAlias maps the GitHub repo name to its runx config
// alias. The general case is identity; only the `llm-cluster-router`
// repo uses a non-matching alias (`router`).
func resolvePublicRepoAlias(githubRepoName string) string {
	if githubRepoName == "llm-cluster-router" {
		return "router"
	}
	return githubRepoName
}

// isHelixonRemote reports whether the push URL targets a Helixon-branded
// repository (helixon-* or helix-dev-tools). When true, the rebrand
// scanner gate blocks pushes containing legacy terms.
func isHelixonRemote(remoteURL string) bool {
	r := strings.ToLower(strings.TrimSpace(remoteURL))
	return strings.Contains(r, "helixon-") || strings.Contains(r, "helix-dev-tools")
}

// readSentruxGateEnabled returns true if the local repo opts in to the
// pre-push sentrux gate via `git config hooks.sentruxGate true`. Defaults
// to false so the gate stays opt-in until baselines are saved everywhere.
// Added in v8900-B5.
func readSentruxGateEnabled() bool {
	cmd := exec.Command("git", "config", "--bool", "hooks.sentruxGate")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}

// evaluateSentruxGateDefault runs `sentrux gate` against the working
// directory. Returns nil when the gate is disabled, when no baseline
// exists, or when the gate passes; returns one or more failure lines
// when the structural gate fails. Added in v8900-B5.
func evaluateSentruxGateDefault() []string {
	cwd, err := os.Getwd()
	if err != nil {
		return []string{fmt.Sprintf("sentrux-gate: cannot determine working directory: %v", err)}
	}
	if _, err := os.Stat(cwd + "/.sentrux/baseline.json"); err != nil {
		return nil
	}
	cmd := exec.Command("sentrux", "gate", ".")
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	body := strings.TrimSpace(string(out))
	if body == "" {
		body = err.Error()
	}
	return []string{
		"sentrux structural gate failed:",
		body,
	}
}

// evaluateRebrandGateDefault scans the repo working directory for legacy
// brand terms when pushing to a helixon-* remote. Returns nil if the
// remote is not a helixon repo or no findings exist.
func evaluateRebrandGateDefault(remoteURL string) []string {
	if !isHelixonRemote(remoteURL) {
		return nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return []string{fmt.Sprintf("rebrand-gate: cannot determine working directory: %v", err)}
	}
	findings, err := scanDirectory(cwd)
	if err != nil {
		return []string{fmt.Sprintf("rebrand-gate: scan error: %v", err)}
	}
	if len(findings) == 0 {
		return nil
	}
	return []string{
		fmt.Sprintf("rebrand-gate: %d legacy term(s) found in repo (run `cursor-tools rebrand scan` for details)", len(findings)),
	}
}

var prePushCmd = &cobra.Command{
	Use:           "pre-push [remote] [url]",
	Short:         "Enforce push policy: identity, public repo gate, rebrand gate, zendesk fast-forward-only, main branch",
	Args:          cobra.RangeArgs(1, 2),
	SilenceUsage:  true,
	SilenceErrors: false,
	RunE:          runPrePush,
}

var protectedBranches = regexp.MustCompile(`^(main|master)$`)

func readAllowMainPush() bool {
	cmd := exec.Command("git", "config", "--bool", "hooks.allowMainPush")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}

func readPrePushStdinLines(stdin io.Reader) ([]string, error) {
	if stdin == nil {
		stdin = os.Stdin
	}
	scanner := bufio.NewScanner(stdin)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func runPrePush(_ *cobra.Command, args []string) error {
	if failures := identityGateEvaluator(identityGateGatherer()); len(failures) > 0 {
		fmt.Fprintln(prePushStderr, "ERROR: cursor-tools identity gate FAILED for personal repo push:")
		for _, f := range failures {
			fmt.Fprintln(prePushStderr, "  - "+f)
		}
		fmt.Fprintln(prePushStderr,
			"\nRemediation:\n"+
				"  unset GITHUB_TOKEN GITHUB_API_TOKEN HOMEBREW_GITHUB_API_TOKEN VENDIR_GITHUB_API_TOKEN\n"+
				"  git config user.email jaslian@gmail.com\n"+
				"  git config user.name 'Jason Lian'\n"+
				"  cursor-tools doctor identity --strict   # confirm gate is green\n"+
				"To opt-out (rare): git config hooks.allowMainPush true (only for main-branch protection bypass; identity gate has no opt-out)")
		prePushExit(1)
		return nil
	}

	remoteURL := ""
	if len(args) >= 2 {
		remoteURL = args[1]
	}

	stdin := prePushStdin
	if stdin == nil {
		stdin = os.Stdin
	}
	stdinLines, err := readPrePushStdinLines(stdin)
	if err != nil {
		fmt.Fprintf(prePushStderr, "ERROR: cursor-tools pre-push could not read hook stdin: %v\n", err)
		prePushExit(1)
		return nil
	}

	if failures := zendeskNonFastForwardFailures(remoteURL, stdinLines); len(failures) > 0 {
		fmt.Fprintln(prePushStderr, "ERROR: cursor-tools blocks non-fast-forward pushes to GitHub zendesk/* remotes (includes --force / history rewrite):")
		for _, f := range failures {
			fmt.Fprintln(prePushStderr, "  - "+f)
		}
		fmt.Fprintln(prePushStderr,
			"\nRemediation:\n"+
				"  Open a PR from a feature branch; do not rewrite published zendesk/* history from a laptop.\n"+
				"  Emergency only: git push --no-verify (disables all pre-push layers; document out-of-band).")
		prePushExit(1)
		return nil
	}

	if findings := publicRepoGateEvaluator(remoteURL); len(findings) > 0 {
		fmt.Fprintln(prePushStderr, "ERROR: cursor-tools public-repo-gate FAILED for public personal repo push:")
		for _, f := range findings {
			fmt.Fprintln(prePushStderr, "  - "+f)
		}
		fmt.Fprintln(prePushStderr,
			"\nRemediation:\n"+
				"  runx public-repo-gate --repo <alias>   # rerun the gate locally\n"+
				"  Edit the offending file(s), or annotate intentional fixtures with:\n"+
				"    // runx-public-repo-gate: allow-file <category>\n"+
				"  See sop/public-repo-sanitization.md for the category taxonomy.\n"+
				"To bypass for one push (rare; document the exemption in the commit body):\n"+
				"  git push --no-verify  # NOTE: this also disables the identity gate")
		prePushExit(1)
		return nil
	}

	if sentruxGateEnabledGetter() {
		if findings := sentruxGateEvaluator(); len(findings) > 0 {
			fmt.Fprintln(prePushStderr, "ERROR: cursor-tools sentrux-gate FAILED for personal repo push:")
			for _, f := range findings {
				fmt.Fprintln(prePushStderr, "  - "+f)
			}
			fmt.Fprintln(prePushStderr,
				"\nRemediation:\n"+
					"  sentrux gate .                 # rerun the gate locally\n"+
					"  Restructure or refactor to clear the regression.\n"+
					"  If the regression is intentional and accepted, save a new baseline\n"+
					"  with `sentrux gate --save .` and commit `.sentrux/baseline.json`.\n"+
					"To bypass for one push (rare; document in commit body):\n"+
					"  git push --no-verify  # NOTE: this also disables all pre-push layers\n"+
					"To disable permanently for this repo:\n"+
					"  git config hooks.sentruxGate false")
			prePushExit(1)
			return nil
		}
	}

	if findings := rebrandGateEvaluator(remoteURL); len(findings) > 0 {
		fmt.Fprintln(prePushStderr, "ERROR: cursor-tools rebrand-gate FAILED for helixon-* repo push:")
		for _, f := range findings {
			fmt.Fprintln(prePushStderr, "  - "+f)
		}
		fmt.Fprintln(prePushStderr,
			"\nRemediation:\n"+
				"  cursor-tools rebrand scan --dir .   # see all findings\n"+
				"  Replace legacy terms (helixon, cursor-tools, cylrl, evomap) with Helixon equivalents.\n"+
				"To bypass for one push (rare; document the exemption in the commit body):\n"+
				"  git push --no-verify  # NOTE: this also disables all pre-push layers")
		prePushExit(1)
		return nil
	}

	if !allowMainPushGetter() {
		for _, line := range stdinLines {
			fields := strings.Fields(line)
			if len(fields) < 3 {
				continue
			}
			remoteRef := fields[2]
			branch := strings.TrimPrefix(remoteRef, "refs/heads/")
			if protectedBranches.MatchString(branch) {
				fmt.Fprintf(prePushStderr,
					"ERROR: direct push to '%s' is blocked.\n"+
						"Use a feature branch and open a pull request.\n"+
						"To opt-out (personal repos): git config hooks.allowMainPush true\n",
					branch)
				prePushExit(1)
				return nil
			}
		}
	}

	return nil
}
