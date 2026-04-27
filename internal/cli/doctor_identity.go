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

var doctorIdentityCmd = &cobra.Command{
	Use:   "identity",
	Short: "Verify personal GitHub identity and poisoned token environment",
	RunE: func(cmd *cobra.Command, _ []string) error {
		failures := evaluateIdentityGate(gatherIdentityGateState())
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
