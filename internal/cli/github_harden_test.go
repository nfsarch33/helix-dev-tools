package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHardenPolicy_DefaultValues(t *testing.T) {
	p := defaultHardenPolicy()
	if p.DefaultBranch != "main" {
		t.Fatalf("default branch: got %q want %q", p.DefaultBranch, "main")
	}
	if p.RequireReviews {
		t.Fatal("require_reviews should default to false")
	}
	if p.RequiredApprovals != 1 {
		t.Fatalf("required_approvals: got %d want 1", p.RequiredApprovals)
	}
	if p.TagPattern != "v*" {
		t.Fatalf("tag_pattern: got %q want %q", p.TagPattern, "v*")
	}
}

func TestHardenPolicy_LoadFromFile(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "policy.yaml")
	data := []byte("default_branch: develop\nrequire_reviews: true\nrequired_approvals: 2\ntag_pattern: release-*\n")
	if err := os.WriteFile(cfg, data, 0o644); err != nil {
		t.Fatal(err)
	}
	p := loadHardenPolicy(cfg)
	if p.DefaultBranch != "develop" {
		t.Fatalf("default_branch: got %q want %q", p.DefaultBranch, "develop")
	}
	if !p.RequireReviews {
		t.Fatal("require_reviews should be true from config")
	}
	if p.RequiredApprovals != 2 {
		t.Fatalf("required_approvals: got %d want 2", p.RequiredApprovals)
	}
	if p.TagPattern != "release-*" {
		t.Fatalf("tag_pattern: got %q want %q", p.TagPattern, "release-*")
	}
}

func TestHardenPolicy_MissingFileReturnsDefaults(t *testing.T) {
	p := loadHardenPolicy("/nonexistent/path/to/policy.yaml")
	if p.DefaultBranch != "main" {
		t.Fatalf("should fall back to default, got %q", p.DefaultBranch)
	}
}

func TestHardenDryRun_AllStepsSkipped(t *testing.T) {
	oldDryRun := hardenDryRun
	hardenDryRun = true
	defer func() { hardenDryRun = oldDryRun }()

	ctx := context.Background()
	policy := defaultHardenPolicy()

	steps := []func(context.Context, string, hardenPolicy) hardenStepResult{
		stepMergeStrategy,
		stepDependabotAlerts,
		stepDependabotSecurityFixes,
		stepSecretScanning,
		stepCodeQL,
		stepActionsToken,
		stepVulnerabilityReporting,
		stepCodeowners,
		stepBranchProtection,
		stepTagProtection,
	}
	for _, fn := range steps {
		r := fn(ctx, "owner/repo", policy)
		if r.Status != "SKIP" {
			t.Fatalf("step %q: expected SKIP in dry-run, got %q", r.Step, r.Status)
		}
		if r.Detail != "dry-run" {
			t.Fatalf("step %q: expected detail 'dry-run', got %q", r.Step, r.Detail)
		}
	}
}

func TestHardenReport_JSONStructure(t *testing.T) {
	report := hardenReport{
		Repo: "owner/repo",
		Results: []hardenStepResult{
			{Step: "merge-strategy", Status: "OK", Detail: "squash+rebase"},
			{Step: "codeql", Status: "WARN", Detail: "not available"},
		},
	}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var decoded hardenReport
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Repo != "owner/repo" {
		t.Fatalf("repo: got %q", decoded.Repo)
	}
	if len(decoded.Results) != 2 {
		t.Fatalf("results count: got %d want 2", len(decoded.Results))
	}
	if decoded.Results[0].Status != "OK" {
		t.Fatalf("first result: got %q want OK", decoded.Results[0].Status)
	}
}

func TestEncodeBase64(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"f", "Zg=="},
		{"fo", "Zm8="},
		{"foo", "Zm9v"},
		{"* @nfsarch33\n", "KiBAbmZzYXJjaDMzCg=="},
	}
	for _, c := range cases {
		got := encodeBase64([]byte(c.input))
		if got != c.want {
			t.Fatalf("encodeBase64(%q): got %q want %q", c.input, got, c.want)
		}
	}
}

func TestHardenWithMockGH(t *testing.T) {
	origAPI := defaultGHAPI
	origGraphQL := defaultGHGraphQL
	defer func() {
		defaultGHAPI = origAPI
		defaultGHGraphQL = origGraphQL
	}()

	var apiCalls []string
	defaultGHAPI = func(_ context.Context, method, endpoint, body string) ([]byte, error) {
		call := fmt.Sprintf("%s %s", method, endpoint)
		apiCalls = append(apiCalls, call)
		if strings.Contains(endpoint, "/contents/.github/CODEOWNERS") && method == "GET" {
			return nil, fmt.Errorf("404")
		}
		return []byte("{}"), nil
	}
	defaultGHGraphQL = func(_ context.Context, query string) ([]byte, error) {
		apiCalls = append(apiCalls, "GRAPHQL")
		return []byte(`{"data":{"repository":{"id":"R_test"},"createBranchProtectionRule":{"branchProtectionRule":{"id":"BPR_test"}}}}`), nil
	}

	oldDryRun := hardenDryRun
	hardenDryRun = false
	defer func() { hardenDryRun = oldDryRun }()

	ctx := context.Background()
	policy := defaultHardenPolicy()
	var buf strings.Builder
	report := hardenOneRepo(ctx, &buf, "owner/testrepo", policy)

	if report.Repo != "owner/testrepo" {
		t.Fatalf("repo: got %q", report.Repo)
	}
	if len(report.Results) != 10 {
		t.Fatalf("expected 10 step results, got %d", len(report.Results))
	}
	for _, r := range report.Results {
		if r.Status != "OK" {
			t.Fatalf("step %q: expected OK, got %q (%s)", r.Step, r.Status, r.Detail)
		}
	}
	if len(apiCalls) == 0 {
		t.Fatal("expected gh api calls to be made")
	}
}

func TestPrintStepResult(t *testing.T) {
	var buf strings.Builder
	printStepResult(&buf, hardenStepResult{Step: "codeql", Status: "OK", Detail: "enabled"})
	out := buf.String()
	if !strings.Contains(out, "OK") || !strings.Contains(out, "codeql") {
		t.Fatalf("unexpected output: %q", out)
	}
}
