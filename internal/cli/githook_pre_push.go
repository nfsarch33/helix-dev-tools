// runx-public-repo-gate: allow-file personal_path_id — identity gate detects literal personal-stack identifiers, so the strings must remain in source

// Package cli — git pre-push handler.
//
// This hook layers three policies:
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
// Policy 2 is the v257 W2 sprint deliverable. Policy 3 is the v317-1
// sprint deliverable. Both are documented in
// `sop/personal-repo-identity.md` and the public-repo sanitisation rule.
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
)

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
	"cursor-tools":          {},
	"fleet-bench":           {},
	"ironclaw-mcp":          {},
	"ironclaw-ops":          {},
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

var prePushCmd = &cobra.Command{
	Use:           "pre-push [remote] [url]",
	Short:         "Block direct pushes to main/master and enforce personal-repo identity gate",
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

	// Policy 3: public-repo sanitisation gate. Runs only when the
	// remote URL points at one of the closed enumeration of public
	// nfsarch33/* repos. Skips silently for private repos and Zendesk
	// work clones.
	remoteURL := ""
	if len(args) >= 2 {
		remoteURL = args[1]
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

	if !allowMainPushGetter() {
		stdin := prePushStdin
		if stdin == nil {
			stdin = os.Stdin
		}
		scanner := bufio.NewScanner(stdin)
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
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
