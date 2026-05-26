
package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var poisonedGitHubTokenEnv = []string{
	"GITHUB_TOKEN",
	"GITHUB_API_TOKEN",
	"HOMEBREW_GITHUB_API_TOKEN",
	"VENDIR_GITHUB_API_TOKEN",
}

type identityGateState struct {
	RemoteURL string
	GitEmail  string
	Env       map[string]string
}

var doctorIdentityFlags struct {
	strict bool
}

var doctorIdentityCmd = &cobra.Command{
	Use:   "identity",
	Short: "Verify personal GitHub identity and poisoned token environment",
	Long: "Verify the local clone's git identity and environment so personal repo pushes never\n" +
		"land with the Zendesk identity or a poisoned GITHUB_TOKEN. With --strict the gate\n" +
		"also fails when the remote is unresolved or the email is empty (defends against\n" +
		"freshly-cloned worktrees that would otherwise fall back to a system default identity).",
	SilenceUsage:  true,
	SilenceErrors: false,
	RunE: func(cmd *cobra.Command, _ []string) error {
		state := gatherIdentityGateState()
		evaluator := evaluateIdentityGate
		if doctorIdentityFlags.strict {
			evaluator = evaluateIdentityGateStrict
		}
		failures := evaluator(state)
		for _, failure := range failures {
			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "FAIL "+failure)
		}
		if len(failures) > 0 {
			return fmt.Errorf("identity failed: %d failure(s)", len(failures))
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "PASS identity")
		return nil
	},
}

func init() {
	doctorIdentityCmd.Flags().BoolVar(&doctorIdentityFlags.strict, "strict", false, "Treat unresolved remote / empty email on personal repo as failures")
	doctorCmd.AddCommand(doctorIdentityCmd)
}

func evaluateIdentityGate(state identityGateState) []string {
	var failures []string
	if isPersonalRemote(state.RemoteURL) {
		for _, key := range poisonedGitHubTokenEnv {
			if strings.TrimSpace(state.Env[key]) != "" {
				failures = append(failures, key+" must be unset for personal repositories")
			}
		}
		email := strings.TrimSpace(strings.ToLower(state.GitEmail))
		expected := personalEmail()
		if expected != "" {
			if email != "" && email != expected && !strings.Contains(email, "noreply.github.com") {
				failures = append(failures, fmt.Sprintf("personal repo git email must be %s or nfsarch33 noreply", expected))
			}
		} else if email != "" && strings.Contains(email, "zendesk") {
			failures = append(failures, "personal repo git email must not be a zendesk address")
		}
	}
	return failures
}

// evaluateIdentityGateStrict layers two extra checks on top of the
// permissive gate so a personal repo push cannot leak the Zendesk
// identity in degraded environments:
//
//  1. Unresolved remote URL (typical of a fresh clone before push) is
//     itself a failure, because we cannot prove the destination is
//     personal-vs-work.
//
//  2. Empty user.email on a personal remote is a failure (the
//     pre-commit hook already covers this for `git commit`; we mirror
//     it for `git push` so a non-default-author commit cannot bypass
//     the gate by skipping the commit hook entirely).
//
// Strict mode never flags Zendesk work clones — it is a personal-repo
// guard, not a global identity policy.
func evaluateIdentityGateStrict(state identityGateState) []string {
	failures := evaluateIdentityGate(state)
	if strings.TrimSpace(state.RemoteURL) == "" {
		failures = append(failures, "strict: unable to resolve git remote URL; run inside a worktree with `origin` configured")
		return failures
	}
	if isPersonalRemote(state.RemoteURL) {
		if strings.TrimSpace(state.GitEmail) == "" {
			failures = append(failures, fmt.Sprintf("strict: personal repo requires explicit user.email (set git config user.email %s)", personalEmailOrPlaceholder()))
		}
	}
	return failures
}

// evaluateIdentityGateForHookSurface adapts the strict gate for the
// IDE hook surface (beforeShellExecution -> guard-shell). The hook
// process inherits its cwd from the IDE, which is often the workspace
// root or a directory that does not contain `origin`. The hook also
// inherits env from the IDE login session, so GITHUB_TOKEN-family vars
// set system-wide cannot be unset for the hook subprocess from inside
// a single Shell call.
//
// The G12 carry-forward (originally scheduled for v267) was hot-fixed
// in the v262 close-out because the original strict-gate logic was
// preemptively denying legitimate `git push` from personal repo
// subdirectories whenever (a) the hook process saw an empty RemoteURL
// AND (b) the inherited env carried a poisoned token. Both conditions
// are normal for IDE-driven development on a developer machine that
// also runs gh CLI, brew, vendir, etc.
//
// Adapted behavior:
//
//  1. If RemoteURL is empty, defer (return zero failures). The actual
//     git/gh command runs from its own cwd, and the per-command gates
//     (commit hooks, repo-local config, SSH key vs token discipline,
//     server-side push protection) own the safety boundary from there.
//     Importantly, `git push` over an SSH origin (which is what every
//     personal repo here uses) does NOT consume GITHUB_TOKEN env vars
//     at all -- they only matter for HTTPS git or the `gh` CLI, which
//     also resolves its target from cwd, not from the token.
//
//  2. If RemoteURL is non-empty, fall through to evaluateIdentityGate
//     so the existing personal-repo + token + email checks still apply
//     with their normal failure messages.
//
// Direct CLI invocation (`cursor-tools doctor identity --strict`) and
// the githook pre-push surface keep using evaluateIdentityGateStrict
// because those surfaces always run from the target repo's own cwd.
func evaluateIdentityGateForHookSurface(state identityGateState) []string {
	if strings.TrimSpace(state.RemoteURL) == "" {
		return nil
	}
	return evaluateIdentityGate(state)
}

func gatherIdentityGateState() identityGateState {
	return identityGateState{
		RemoteURL: gitOutput("remote", "get-url", "origin"),
		GitEmail:  gitOutput("config", "user.email"),
		Env:       selectedEnv(poisonedGitHubTokenEnv),
	}
}

func isPersonalRemote(remote string) bool {
	remote = strings.ToLower(remote)
	if strings.Contains(remote, "github.com:nfsarch33/") ||
		strings.Contains(remote, "github.com/nfsarch33/") {
		return true
	}
	if alias := os.Getenv("RUNX_PERSONAL_SSH_HOST"); alias != "" {
		return strings.Contains(remote, strings.ToLower(alias)+":nfsarch33/")
	}
	return false
}

func selectedEnv(keys []string) map[string]string {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		out[key] = os.Getenv(key)
	}
	return out
}

func gitOutput(args ...string) string {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
