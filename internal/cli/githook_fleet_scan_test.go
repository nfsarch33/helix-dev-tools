package cli

import (
	"strings"
	"testing"
)

// TestFleetScan_RejectsForbiddenZDURL drives the
// `cursor-tools githook pre-commit-fleet-scan` surface. A staged file
// path that lives under a fleet-controlled tree must not contain a
// hard-coded reference to the Zendesk AI gateway. The plan boundary is
// "MacBook only ZD AI gateway"; fleet code must never embed the URL.
func TestFleetScan_RejectsForbiddenZDURL(t *testing.T) {
	cases := []struct {
		name     string
		path     string
		content  string
		expected bool
	}{
		{
			"fleet code with forbidden ZD URL",
			"go/internal/mc/delegate.go",
			"const gw = \"https://api.cursor-ai.zendesk.com/v1/chat\"\n",
			true,
		},
		{
			"fleet code with forbidden cursor-ai.zendesk subdomain",
			"go/internal/router/wsl1.go",
			"// upstream: https://gateway.cursor-ai.zendesk.com\n",
			true,
		},
		{
			"docs file mentioning the URL",
			"docs/strategy.md",
			"The MacBook talks to https://api.cursor-ai.zendesk.com/.\n",
			false,
		},
		{
			"fleet code clean",
			"go/internal/mc/delegate.go",
			"const gw = \"http://127.0.0.1:9787/v1/chat\"\n",
			false,
		},
		{
			"non-fleet file with forbidden URL",
			"local-notes.md",
			"https://api.cursor-ai.zendesk.com\n",
			false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			finding := scanFleetForbiddenURLs(c.path, []byte(c.content))
			got := finding != ""
			if got != c.expected {
				t.Fatalf("scanFleetForbiddenURLs(%q):\n  content=%q\n  got finding=%q (expected match=%v)",
					c.path, c.content, finding, c.expected)
			}
		})
	}
}

// TestFleetScan_FleetPathsAreCanonical pins the fleet root prefixes so a
// developer cannot quietly broaden the deny scope. Fleet code lives
// under three roots in this repo set:
//   - go/internal/cylrl, go/internal/mc, go/internal/router (cylrl
//     orchestrator + MC + router)
//   - ironclaw-ops/ (deployment overlays)
//   - ironclaw/ (engine prod code)
func TestFleetScan_FleetPathsAreCanonical(t *testing.T) {
	want := []string{
		"go/internal/cylrl/",
		"go/internal/mc/",
		"go/internal/router/",
		"ironclaw-ops/",
		"ironclaw/",
	}
	got := fleetCodePathPrefixes()
	if len(got) != len(want) {
		t.Fatalf("fleet path prefixes mismatch: got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("fleet path[%d]: got=%q want=%q", i, got[i], want[i])
		}
	}
}

// TestFleetScan_RunStreamFlagsAndExits ensures the orchestrator reads
// the staged-file list, scans each one, and surfaces a non-empty error
// summary when at least one fleet file leaks the gateway URL.
func TestFleetScan_RunStreamFlagsAndExits(t *testing.T) {
	stagedFiles := []stagedFileScan{
		{Path: "go/internal/mc/delegate.go", Content: []byte("// good\n")},
		{Path: "go/internal/router/wsl1.go", Content: []byte("const gw = \"https://gateway.cursor-ai.zendesk.com\"\n")},
		{Path: "docs/strategy.md", Content: []byte("# notes about https://api.cursor-ai.zendesk.com\n")},
	}
	findings := runFleetScan(stagedFiles)
	if len(findings) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %#v", len(findings), findings)
	}
	if !strings.Contains(findings[0], "go/internal/router/wsl1.go") {
		t.Fatalf("expected finding to name fleet path, got: %s", findings[0])
	}
	if !strings.Contains(findings[0], "cursor-ai.zendesk.com") {
		t.Fatalf("expected finding to name forbidden URL, got: %s", findings[0])
	}
}
