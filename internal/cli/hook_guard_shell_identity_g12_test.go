// runx-public-repo-gate: allow-file personal_path_id,fleet_host_alias — tests assert detection of literal personal-stack identifiers (gate test fixtures)

package cli

import (
	"strings"
	"testing"
)

// G12 carry-forward (hot-fixed in v262 close-out): the IDE hook surface
// must not preemptively deny sensitive shell commands when its OWN cwd
// has no resolvable git origin. The hook process inherits cwd from the
// IDE, which often does not match the actual command's target cwd. The
// safety boundary is preserved because the actual git/gh command still
// runs under its own cwd's identity (commit hooks, repo-local config,
// SSH key vs token discipline). Empty RemoteURL on the hook surface is
// therefore treated as "indeterminate -> defer", not "fail".
//
// Scope of this hot-fix: hook surface only. Direct CLI invocation of
// `cursor-tools doctor identity --strict` and `githook pre-push` keep
// the original strict semantics so freshly-cloned worktrees still get
// gated at the per-command surface they intentionally guard.

func TestEvaluateIdentityGateForHookSurface_DefersOnEmptyRemote(t *testing.T) {
	t.Parallel()

	// Hook process cwd has no git origin (typical when the IDE invokes
	// the hook from the workspace root which is itself not a git repo,
	// or from $HOME during freshly opened windows). Even with the full
	// poisoned token family set, the hook must defer rather than deny:
	// the actual `git push` runs from its real cwd which is responsible
	// for its own identity discipline.
	failures := evaluateIdentityGateForHookSurface(identityGateState{
		RemoteURL: "",
		GitEmail:  "jaslian@gmail.com",
		Env: map[string]string{
			"GITHUB_TOKEN":              "ghp_anything",
			"GITHUB_API_TOKEN":          "ghp_anything",
			"HOMEBREW_GITHUB_API_TOKEN": "ghp_anything",
			"VENDIR_GITHUB_API_TOKEN":   "ghp_anything",
		},
	})
	if len(failures) != 0 {
		t.Fatalf("hook surface must defer on empty RemoteURL, got failures: %v", failures)
	}
}

func TestEvaluateIdentityGateForHookSurface_FailsOnPersonalRemoteWithPoisonedToken(t *testing.T) {
	t.Parallel()

	// Hook cwd CAN see a personal remote -> existing safety boundary
	// still applies: any poisoned GITHUB_TOKEN-family var must trigger
	// a fail with an actionable message naming the offending var.
	failures := evaluateIdentityGateForHookSurface(identityGateState{
		RemoteURL: "git@github.com:nfsarch33/cursor-tools.git",
		GitEmail:  "jaslian@gmail.com",
		Env: map[string]string{
			"GITHUB_TOKEN": "ghp_anything",
		},
	})
	if len(failures) == 0 {
		t.Fatalf("hook surface must still fail when personal remote + poisoned token are both visible")
	}
	if !strings.Contains(strings.Join(failures, "\n"), "GITHUB_TOKEN") {
		t.Fatalf("failure must name GITHUB_TOKEN: %v", failures)
	}
}

func TestEvaluateIdentityGateForHookSurface_PassesOnPersonalRemoteWithCleanEnv(t *testing.T) {
	t.Parallel()

	failures := evaluateIdentityGateForHookSurface(identityGateState{
		RemoteURL: "git@github-agtc:nfsarch33/cursor-global-kb.git",
		GitEmail:  "jaslian@gmail.com",
		Env:       map[string]string{},
	})
	if len(failures) != 0 {
		t.Fatalf("hook surface should pass on personal remote with clean env, got %v", failures)
	}
}

func TestEvaluateIdentityGateForHookSurface_DoesNotApplyToWorkRemote(t *testing.T) {
	t.Parallel()

	// Zendesk work clones use the work identity by design. Even on the
	// hook surface, work-remote pushes must not be gated by the
	// personal-repo strict checks.
	failures := evaluateIdentityGateForHookSurface(identityGateState{
		RemoteURL: "git@github.com:zendesk/secure-auth-platform.git",
		GitEmail:  "jlianzendesk@zendesk.com",
		Env:       map[string]string{"GITHUB_TOKEN": "ghp_work"},
	})
	if len(failures) != 0 {
		t.Fatalf("work remote must not be gated by hook surface, got %v", failures)
	}
}

// TestIdentityStrictShellDeny_DefersWhenHookCwdHasNoOrigin is the
// integration-shape test for the bug that motivated the hot-fix:
// `git push` from a personal repo subdir was denied at the hook surface
// because the hook process ran from a cwd without origin AND the IDE's
// inherited env still had GITHUB_TOKEN family set.
func TestIdentityStrictShellDeny_DefersWhenHookCwdHasNoOrigin(t *testing.T) {
	prev := identityGateStateProviderForTest
	t.Cleanup(func() { identityGateStateProviderForTest = prev })
	identityGateStateProviderForTest = func() identityGateState {
		return identityGateState{
			RemoteURL: "",
			GitEmail:  "jaslian@gmail.com",
			Env: map[string]string{
				"GITHUB_TOKEN":              "ghp_set_in_login_env",
				"GITHUB_API_TOKEN":          "ghp_set_in_login_env",
				"HOMEBREW_GITHUB_API_TOKEN": "ghp_set_in_login_env",
				"VENDIR_GITHUB_API_TOKEN":   "ghp_set_in_login_env",
			},
		}
	}

	resp := identityStrictShellDeny("git push origin v262-cursor-tools-offload-mcp")
	if resp != nil {
		t.Fatalf("hook surface must defer (return nil) when cwd has no origin even with poisoned env, got deny: %#v", resp)
	}
}

// TestIdentityStrictShellDeny_DeniesPersonalRemoteWithPoisonedToken
// guards the safety property the hot-fix must preserve.
func TestIdentityStrictShellDeny_DeniesPersonalRemoteWithPoisonedToken(t *testing.T) {
	prev := identityGateStateProviderForTest
	t.Cleanup(func() { identityGateStateProviderForTest = prev })
	identityGateStateProviderForTest = func() identityGateState {
		return identityGateState{
			RemoteURL: "git@github.com:nfsarch33/cursor-tools.git",
			GitEmail:  "jaslian@gmail.com",
			Env:       map[string]string{"GITHUB_TOKEN": "ghp_set"},
		}
	}

	resp := identityStrictShellDeny("git push origin v262-cursor-tools-offload-mcp")
	if resp == nil {
		t.Fatalf("hook surface MUST deny when personal remote + GITHUB_TOKEN are both visible")
	}
	if !strings.Contains(resp.UserMessage, "GITHUB_TOKEN") {
		t.Fatalf("deny message must name GITHUB_TOKEN: %q", resp.UserMessage)
	}
}
