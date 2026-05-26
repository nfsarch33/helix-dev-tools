// runx-public-repo-gate: allow-file personal_path_id,fleet_host_alias — tests assert detection of literal personal-stack identifiers (gate test fixtures)

package cli

import (
	"sort"
	"strings"
	"testing"
)

// TestIdentityStrictSurfaces_RegisterCanonicalFour pins the surface
// registry so a new contributor cannot quietly delete a strict-identity
// gate. The four surfaces are:
//
//   - wrapper:        the `cursor-tools` shell wrapper (bin/cursor-tools)
//   - daily-startup:  the `cursor-tools daily-refresh` cobra command
//   - pre-push:       the `cursor-tools githook pre-push` git hook
//   - ide-hook:       the Cursor `~/.cursor/hooks.json` beforeShellExecution chain
//
// Each surface has a stable id, a human-readable description, and a
// recommended remediation snippet. Tests assert the registry is exactly
// the canonical four; deletions/renames break the test.
func TestIdentityStrictSurfaces_RegisterCanonicalFour(t *testing.T) {
	got := IdentityStrictSurfaces()
	wantIDs := []string{"daily-startup", "ide-hook", "pre-push", "wrapper"}

	gotIDs := make([]string, 0, len(got))
	seen := map[string]bool{}
	for _, s := range got {
		if seen[s.ID] {
			t.Fatalf("duplicate surface id %q", s.ID)
		}
		seen[s.ID] = true
		gotIDs = append(gotIDs, s.ID)
		if s.Description == "" {
			t.Fatalf("surface %q has empty description", s.ID)
		}
		if !strings.Contains(s.Remediation, "doctor identity --strict") {
			t.Fatalf("surface %q remediation must reference `doctor identity --strict`, got: %s", s.ID, s.Remediation)
		}
	}
	sort.Strings(gotIDs)
	if !equalStrings(gotIDs, wantIDs) {
		t.Fatalf("surfaces mismatch:\n  got=%v\n  want=%v", gotIDs, wantIDs)
	}
}

// TestRunIdentityStrictGate_BlocksOnFailure ensures the daily-refresh
// shim invokes the strict gate and surfaces failures back to the caller.
func TestRunIdentityStrictGate_BlocksOnFailure(t *testing.T) {
	state := identityGateState{
		RemoteURL: "git@github.com:nfsarch33/ai-agent-business-stack.git",
		GitEmail:  "user@example.com",
		Env: map[string]string{
			"GITHUB_TOKEN": "leaked-token",
		},
	}
	if err := runIdentityStrictGateWithState(state); err == nil {
		t.Fatalf("expected gate to fail when GITHUB_TOKEN is set on a personal remote")
	}
}

func TestRunIdentityStrictGate_PassesOnClean(t *testing.T) {
	state := identityGateState{
		RemoteURL: "git@github.com:nfsarch33/ai-agent-business-stack.git",
		GitEmail:  "user@example.com",
		Env:       map[string]string{},
	}
	if err := runIdentityStrictGateWithState(state); err != nil {
		t.Fatalf("expected gate to pass on clean state, got %v", err)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
