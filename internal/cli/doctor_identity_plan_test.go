
package cli

import "testing"

func TestEvaluateIdentityGate_RejectsPoisonedGitHubTokens(t *testing.T) {
	t.Parallel()

	failures := evaluateIdentityGate(identityGateState{
		RemoteURL: "git@github.com:nfsarch33/ai-agent-business-stack.git",
		GitEmail:  "user@example.com",
		Env: map[string]string{
			"GITHUB_TOKEN":              "ghp_work",
			"GITHUB_API_TOKEN":          "ghp_work",
			"HOMEBREW_GITHUB_API_TOKEN": "ghp_work",
			"VENDIR_GITHUB_API_TOKEN":   "ghp_work",
		},
	})

	if len(failures) != 4 {
		t.Fatalf("expected four token failures, got %d: %v", len(failures), failures)
	}
}

func TestEvaluateIdentityGate_RejectsZendeskEmailOnPersonalRemote(t *testing.T) {
	t.Parallel()

	failures := evaluateIdentityGate(identityGateState{
		RemoteURL: "git@github.com:nfsarch33/cursor-global-kb.git",
		GitEmail:  "jason.lian@zendesk.com",
		Env:       map[string]string{},
	})

	if len(failures) != 1 {
		t.Fatalf("expected personal email failure, got %v", failures)
	}
}

func TestEvaluateIdentityGate_AllowsPersonalIdentity(t *testing.T) {
	t.Parallel()

	failures := evaluateIdentityGate(identityGateState{
		RemoteURL: "git@github.com:nfsarch33/cursor-global-kb.git",
		GitEmail:  "user@example.com",
		Env:       map[string]string{},
	})

	if len(failures) != 0 {
		t.Fatalf("expected no failures, got %v", failures)
	}
}

func TestDoctorIdentityCommandRegistered(t *testing.T) {
	t.Parallel()

	names := []string{}
	for _, cmd := range doctorCmd.Commands() {
		names = append(names, cmd.Name())
	}
	if !containsString(names, "identity") {
		t.Fatalf("doctor identity command not registered; got %v", names)
	}
}
