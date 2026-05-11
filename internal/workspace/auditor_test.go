package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRunxRepos_ExpandsHomeAndFiltersAliases(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, "config.yaml")
	data := []byte(`repos:
  global-kb:
    path: "$HOME/Code/global-kb"
    identity: nfsarch33
    default_branch: main
  business:
    path: "~/ai-agent-business-stack"
    identity: nfsarch33
`)
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	repos, err := LoadRunxRepos(configPath, home, []string{"business"})
	if err != nil {
		t.Fatalf("LoadRunxRepos: %v", err)
	}
	if len(repos) != 1 {
		t.Fatalf("repos len = %d, want 1", len(repos))
	}
	if repos[0].Alias != "business" {
		t.Fatalf("alias = %q, want business", repos[0].Alias)
	}
	if !strings.HasPrefix(repos[0].Path, home) {
		t.Fatalf("path %q does not use test home %q", repos[0].Path, home)
	}
	if repos[0].DefaultBranch != "main" {
		t.Fatalf("default branch = %q, want main", repos[0].DefaultBranch)
	}
}

func TestAuditor_FindsDirtyAndUnpushedRepo(t *testing.T) {
	runner := RunnerFunc(func(_ context.Context, dir string, args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "status --porcelain":
			return " M file.go\n?? scratch.txt\n", nil
		case "rev-parse --abbrev-ref HEAD":
			return "feature\n", nil
		case "rev-parse --abbrev-ref --symbolic-full-name @{u}":
			return "origin/feature\n", nil
		case "rev-list --count @{u}..HEAD":
			return "2\n", nil
		case "rev-list --count HEAD..@{u}":
			return "0\n", nil
		case "branch -vv":
			return "* feature abc123 [origin/feature: gone] work\n", nil
		default:
			if dir == "" {
				t.Fatal("empty dir")
			}
			return "", nil
		}
	})

	report, err := NewAuditor(runner).Audit(context.Background(), AuditOptions{
		Repos: []RepoConfig{{Alias: "business", Path: t.TempDir(), DefaultBranch: "main"}},
		Quick: true,
	})
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	got := report.Repos[0].FindingCodes()
	for _, want := range []FindingCode{FindingDirtyWorktree, FindingUnpushedCommits, FindingStaleTrackingRef} {
		if !containsFinding(got, want) {
			t.Fatalf("findings = %#v, missing %s", got, want)
		}
	}
}

func containsFinding(values []FindingCode, want FindingCode) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// TestIsKnownVendorMirror_CanonicalList pins the vendor classification
// against the canonical list documented in
// global-memories/daily-startup-prompt.md: ironclaw, openclaw,
// temporal, gstack, hermes are vendor mirrors and stay analysis-only
// unless their repo rules say otherwise. The Workspace Doctor uses
// this list to demote `behind_default` findings on these repos to
// `vendor_behind` (info, not warning), which keeps them out of the
// RED hard-stop tier when upstream advances.
func TestIsKnownVendorMirror_CanonicalList(t *testing.T) {
	cases := []struct {
		alias string
		want  bool
	}{
		{"ironclaw", true},
		{"openclaw", true},
		{"hermes", true},
		{"gstack", true},
		{"temporal", true},
		{"windows-mcp", true},
		{"global-kb", false},
		{"runx", false},
		{"business", false},
		{"ironclaw-mcp", false},
		{"ironclaw-ops", false},
		{"openclaw-mission-control", false},
		{"mission-control", false},
	}
	for _, tc := range cases {
		got := isKnownVendorMirror(tc.alias)
		if got != tc.want {
			t.Errorf("isKnownVendorMirror(%q) = %v; want %v", tc.alias, got, tc.want)
		}
	}
}
