package cli

import (
	"strings"

	"github.com/nfsarch33/cursor-tools/internal/hookio"
)

// identityGateStateProviderForTest is the seam tests use to inject a
// fake identityGateState. Production code path is gatherIdentityGateState.
var identityGateStateProviderForTest func() identityGateState

// shellSensitiveCommandPrefixes lists the command starts that, when
// executed from the IDE, MUST go through the strict identity gate
// before we let `guard-shell` allow the call.
//
// The matching is intentionally substring-based ("git push" anywhere in
// the command, including after `&&` / `;`) because Cursor frequently
// dispatches compound shell pipelines.
var shellSensitiveCommandPatterns = []string{
	"git push",
	"gh pr create",
	"gh pr edit",
	"gh repo clone",
	"gh repo create",
	"cursor-tools daily-refresh",
	"cursor-tools githook pre-push",
}

// shellIdentityCommandIsSensitive returns true when the supplied shell
// command is one we MUST front-stop with the strict identity gate.
func shellIdentityCommandIsSensitive(cmd string) bool {
	c := strings.TrimSpace(cmd)
	if c == "" {
		return false
	}
	for _, pat := range shellSensitiveCommandPatterns {
		if strings.Contains(c, pat) {
			return true
		}
	}
	return false
}

// identityStrictShellDeny is the IDE-hook surface counterpart to
// strictFleetPreflightDeny: it returns a deny response when the
// command is identity-sensitive AND the gate fails for the hook
// surface. A nil return means "this surface had nothing to add; let
// the rest of the guard-shell pipeline run".
//
// G12 hot-fix: this surface uses evaluateIdentityGateForHookSurface
// (not evaluateIdentityGateStrict) because the hook process cwd often
// does not match the actual command's target cwd, and the IDE login
// env routinely carries GITHUB_TOKEN-family vars. The hook-surface
// evaluator defers on indeterminate cwd while keeping the safety
// property when the cwd does resolve to a personal remote. See
// doctor_identity.go for the full rationale.
func identityStrictShellDeny(cmd string) *hookio.Response {
	if !shellIdentityCommandIsSensitive(cmd) {
		return nil
	}
	state := gatherIdentityGateStateForGuardShell()
	failures := evaluateIdentityGateForHookSurface(state)
	if len(failures) == 0 {
		return nil
	}
	user := "BLOCKED: strict identity gate failed before sensitive shell command\n  - " +
		strings.Join(failures, "\n  - ")
	agent := "Run `cursor-tools doctor identity --strict` to see the same failures and the canonical remediation. " +
		"Do NOT bypass — fix the identity (unset GITHUB_TOKEN family / set personal user.email) and rerun the command."
	return hookio.Deny(user, agent)
}

// gatherIdentityGateStateForGuardShell prefers the test seam when set
// so guard-shell unit tests stay hermetic without shelling out to git.
func gatherIdentityGateStateForGuardShell() identityGateState {
	if identityGateStateProviderForTest != nil {
		return identityGateStateProviderForTest()
	}
	return gatherIdentityGateState()
}
