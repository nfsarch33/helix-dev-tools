// runx-public-repo-gate: allow-file personal_path_id,fleet_host_alias — tests assert detection of literal personal-stack identifiers (gate test fixtures)

package cli

import (
	"strings"
	"testing"
)

// TestEvaluateIdentityGate_StrictWithMissingRemoteFails captures the
// requirement that --strict must FAIL when the remote URL cannot be
// resolved (because we cannot prove it is or is not a personal repo).
// In permissive mode, an empty remote is allowed because the developer
// might be running cursor-tools doctor identity outside a git worktree
// (e.g. on a fresh box during onboarding).
func TestEvaluateIdentityGate_StrictWithMissingRemoteFails(t *testing.T) {
	t.Parallel()

	failures := evaluateIdentityGateStrict(identityGateState{
		RemoteURL: "",
		GitEmail:  "jaslian@gmail.com",
		Env:       map[string]string{},
	})
	if len(failures) == 0 {
		t.Fatalf("strict mode must fail on missing remote, got 0 failures")
	}
	joined := strings.Join(failures, "\n")
	if !strings.Contains(joined, "remote") {
		t.Fatalf("strict failure should mention remote: %q", joined)
	}
}

// TestEvaluateIdentityGate_StrictWithEmptyEmailOnPersonalRemoteFails
// guards against the `user.email == ""` case which the pre-commit hook
// already catches. The strict identity gate must also catch it on
// pre-push, otherwise a freshly-cloned worktree without an email could
// push as the system default identity (often the Zendesk identity).
func TestEvaluateIdentityGate_StrictWithEmptyEmailOnPersonalRemoteFails(t *testing.T) {
	t.Parallel()

	failures := evaluateIdentityGateStrict(identityGateState{
		RemoteURL: "git@github-agtc:nfsarch33/cursor-global-kb.git",
		GitEmail:  "",
		Env:       map[string]string{},
	})
	if len(failures) == 0 {
		t.Fatalf("strict mode must fail on empty email + personal remote")
	}
}

// TestEvaluateIdentityGate_StrictAllowsCleanIdentity is the happy path:
// personal remote, personal email, no poisoned tokens.
func TestEvaluateIdentityGate_StrictAllowsCleanIdentity(t *testing.T) {
	t.Parallel()

	failures := evaluateIdentityGateStrict(identityGateState{
		RemoteURL: "git@github-agtc:nfsarch33/cursor-global-kb.git",
		GitEmail:  "jaslian@gmail.com",
		Env:       map[string]string{},
	})
	if len(failures) != 0 {
		t.Fatalf("strict mode should pass on clean identity, got %v", failures)
	}
}

// TestEvaluateIdentityGate_StrictDoesNotApplyToWorkRemote ensures the
// strict gate is a no-op on Zendesk work clones. Work clones live under
// ~/Code/secure-auth-platform and use the work identity by design;
// strict mode must not break those.
func TestEvaluateIdentityGate_StrictDoesNotApplyToWorkRemote(t *testing.T) {
	t.Parallel()

	failures := evaluateIdentityGateStrict(identityGateState{
		RemoteURL: "git@github.com:zendesk/secure-auth-platform.git",
		GitEmail:  "jlianzendesk@zendesk.com",
		Env:       map[string]string{"GITHUB_TOKEN": "ghp_work"},
	})
	if len(failures) != 0 {
		t.Fatalf("work remote must not be gated by strict identity, got %v", failures)
	}
}
