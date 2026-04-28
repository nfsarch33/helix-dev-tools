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
		if email != "" && email != "jaslian@gmail.com" && !strings.Contains(email, "noreply.github.com") {
			failures = append(failures, "personal repo git email must be jaslian@gmail.com or nfsarch33 noreply")
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
			failures = append(failures, "strict: personal repo requires explicit user.email (set git config user.email jaslian@gmail.com)")
		}
	}
	return failures
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
	return strings.Contains(remote, "github-agtc:nfsarch33/") ||
		strings.Contains(remote, "github.com:nfsarch33/") ||
		strings.Contains(remote, "github.com/nfsarch33/")
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
