// runx-public-repo-gate: allow-file personal_path_id,fleet_host_alias — tests assert detection of literal personal-stack identifiers (gate test fixtures)

package cli

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/hookio"
	"github.com/nfsarch33/helix-dev-tools/internal/logger"
	"github.com/nfsarch33/helix-dev-tools/internal/patterns"
)

// shellIdentityCommandIsSensitive is the contract under test: it must
// return true for any shell command that would push code or trigger a
// fleet refresh on personal repos. The IDE-hook (beforeShellExecution)
// surface uses this to short-circuit work when GITHUB_TOKEN is leaked
// into the Cursor session.
func TestShellIdentityCommandIsSensitive_TableDriven(t *testing.T) {
	t.Helper()

	cases := []struct {
		name string
		cmd  string
		want bool
	}{
		{name: "empty", cmd: "", want: false},
		{name: "ls", cmd: "ls -la", want: false},
		{name: "git status", cmd: "git status --porcelain", want: false},
		{name: "git log", cmd: "git log --oneline -10", want: false},
		{name: "git push", cmd: "git push", want: true},
		{name: "git push origin", cmd: "git push origin main", want: true},
		{name: "git push --force-with-lease", cmd: "git push --force-with-lease", want: true},
		{name: "gh pr create", cmd: "gh pr create --title foo", want: true},
		{name: "gh repo clone", cmd: "gh repo clone nfsarch33/cylrl-orchestrator", want: true},
		{name: "cursor-tools daily-refresh", cmd: "cursor-tools daily-refresh --skip-sync", want: true},
		{name: "cursor-tools githook pre-push", cmd: "cursor-tools githook pre-push", want: true},
		{name: "cursor-tools doctor", cmd: "cursor-tools doctor identity --strict", want: false},
		{name: "echo unrelated", cmd: "echo 'hello world'", want: false},
		// Embedded inside a longer pipeline the sensitivity must still trip:
		{name: "compound git push", cmd: "make build && git push origin HEAD", want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := shellIdentityCommandIsSensitive(tc.cmd)
			if got != tc.want {
				t.Fatalf("shellIdentityCommandIsSensitive(%q) = %v, want %v", tc.cmd, got, tc.want)
			}
		})
	}
}

// guardShellIdentityDenyOnLeaked is the IDE-hook integration: when a
// sensitive command is paired with a poisoned identityGateState, the
// guard MUST deny with a remediation hint that points operators at the
// strict gate. When the gate is clean (no failures), the guard MUST
// fall through to the pattern matcher (allow / pattern-driven deny).
func TestGuardShell_DeniesSensitiveCommandWhenIdentityPoisoned(t *testing.T) {
	tmp := t.TempDir()
	m, err := patterns.NewMatcher(patterns.ShellDenyPatterns, patterns.ShellWarnPatterns)
	if err != nil {
		t.Fatalf("compile matcher: %v", err)
	}
	h := &guardShellHandler{
		matcher:     m,
		log:         logger.New(filepath.Join(tmp, "ide-hook.log")),
		metricsPath: filepath.Join(tmp, "metrics.jsonl"),
	}

	// Inject a poisoned state. The test-host-1 personal repo + a leaked
	// GITHUB_TOKEN is the canonical regression we want to catch
	// before `git push` ever runs.
	prev := identityGateStateProviderForTest
	identityGateStateProviderForTest = func() identityGateState {
		return identityGateState{
			RemoteURL: "git@github.com:nfsarch33/cursor-global-kb.git",
			GitEmail:  "user@example.com",
			Env: map[string]string{
				"GITHUB_TOKEN": "leaked-zendesk-token",
			},
		}
	}
	defer func() { identityGateStateProviderForTest = prev }()

	resp, err := h.Handle(context.Background(), &hookio.Input{Command: "git push origin main"})
	if err != nil {
		t.Fatalf("Handle returned err: %v", err)
	}
	if resp.Permission != "deny" {
		t.Fatalf("expected deny, got %s (user=%q agent=%q)", resp.Permission, resp.UserMessage, resp.AgentMessage)
	}
	combined := resp.UserMessage + " " + resp.AgentMessage
	if want := "doctor identity --strict"; !strings.Contains(combined, want) {
		t.Fatalf("expected remediation hint to mention %q, got user=%q agent=%q",
			want, resp.UserMessage, resp.AgentMessage)
	}
	if want := "runx pr create"; !strings.Contains(resp.AgentMessage, want) {
		t.Fatalf("expected agent message to cite %q, got %q", want, resp.AgentMessage)
	}
	if want := "personal-repo-shell-hygiene"; !strings.Contains(resp.AgentMessage, want) {
		t.Fatalf("expected agent message to cite skill %q, got %q", want, resp.AgentMessage)
	}
	if bad := "fix the identity (unset GITHUB_TOKEN"; strings.Contains(resp.AgentMessage, bad) {
		t.Fatalf("agent message must not suggest legacy unset pattern")
	}
}

func TestGuardShell_AllowsSensitiveCommandWhenIdentityClean(t *testing.T) {
	tmp := t.TempDir()
	m, err := patterns.NewMatcher(patterns.ShellDenyPatterns, patterns.ShellWarnPatterns)
	if err != nil {
		t.Fatalf("compile matcher: %v", err)
	}
	h := &guardShellHandler{
		matcher:     m,
		log:         logger.New(filepath.Join(tmp, "ide-hook.log")),
		metricsPath: filepath.Join(tmp, "metrics.jsonl"),
	}
	prev := identityGateStateProviderForTest
	identityGateStateProviderForTest = func() identityGateState {
		return identityGateState{
			RemoteURL: "git@github.com:nfsarch33/cursor-global-kb.git",
			GitEmail:  "user@example.com",
			Env:       map[string]string{},
		}
	}
	defer func() { identityGateStateProviderForTest = prev }()

	resp, err := h.Handle(context.Background(), &hookio.Input{Command: "git push origin main"})
	if err != nil {
		t.Fatalf("Handle returned err: %v", err)
	}
	if resp.Permission != "allow" {
		t.Fatalf("expected allow, got %s (user=%q agent=%q)", resp.Permission, resp.UserMessage, resp.AgentMessage)
	}
}
