package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRebrandRules_CoverAllCategories(t *testing.T) {
	seen := map[rebrandCategory]bool{}
	for _, r := range rebrandRules {
		seen[r.Category] = true
	}
	want := []rebrandCategory{
		catBrandName, catToolName, catDeprecated,
		catEnvVar, catK8sLabel, catDockerImage, catGoModule,
	}
	for _, c := range want {
		if !seen[c] {
			t.Fatalf("rebrandRules missing category %q", c)
		}
	}
}

func TestScanFile_DetectsLegacyTerms(t *testing.T) {
	dir := t.TempDir()
	content := `package main

import "github.com/nfsarch33/ironclaw-mcp/pkg/server"

const ns = "ironclaw-system"
var img = "ironclaw/agent:latest"
var old = "cylrl-system"
var prefix = "IRONCLAW_API_KEY"
`
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	findings, err := scanFile(path, "main.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) < 4 {
		t.Fatalf("expected at least 4 findings, got %d: %+v", len(findings), findings)
	}

	categories := map[rebrandCategory]bool{}
	for _, f := range findings {
		categories[f.Category] = true
		if f.File != "main.go" {
			t.Fatalf("expected file 'main.go', got %q", f.File)
		}
		if f.Replacement == "" {
			t.Fatalf("finding %+v has empty replacement", f)
		}
	}
	if !categories[catGoModule] {
		t.Fatal("expected go-module-path category in findings")
	}
}

func TestScanFile_NoFalsePositives(t *testing.T) {
	dir := t.TempDir()
	content := `package main

import "github.com/nfsarch33/helixon-ops/pkg/server"

const ns = "helixon-system"
var prefix = "HELIXON_API_KEY"
`
	path := filepath.Join(dir, "clean.go")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	findings, err := scanFile(path, "clean.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings in clean file, got %d: %+v", len(findings), findings)
	}
}

func TestScanFile_SkipsBinaryExtensions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "image.png")
	if err := os.WriteFile(path, []byte("ironclaw binary content"), 0o644); err != nil {
		t.Fatal(err)
	}

	findings, err := scanFile(path, "image.png")
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("binary files should be skipped, got %d findings", len(findings))
	}
}

func TestScanFile_DetectsCaseVariations(t *testing.T) {
	dir := t.TempDir()
	content := "IronClaw is the old name\nIRONCLAW_MODE=true\nironclaw-ops deploys here\n"
	path := filepath.Join(dir, "readme.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	findings, err := scanFile(path, "readme.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 3 {
		t.Fatalf("expected 3 case-variant findings, got %d: %+v", len(findings), findings)
	}
}

func TestScanFile_DetectsDeprecatedNames(t *testing.T) {
	dir := t.TempDir()
	content := "cylrl orchestrator config\nEvoMap pattern tracker\nCYLRL_TIMEOUT=30\n"
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	findings, err := scanFile(path, "config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) < 3 {
		t.Fatalf("expected at least 3 deprecated-name findings, got %d: %+v", len(findings), findings)
	}
}

func TestIsBinaryPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"main.go", false},
		{"image.png", true},
		{"icon.ico", true},
		{"README.md", false},
		{"lib.so", true},
		{"data.json", false},
		{"bundle.wasm", true},
	}
	for _, c := range cases {
		got := isBinaryPath(c.path)
		if got != c.want {
			t.Fatalf("isBinaryPath(%q): got %v want %v", c.path, got, c.want)
		}
	}
}

func TestScanDirectory_Walk(t *testing.T) {
	dir := t.TempDir()

	// Create files with legacy terms
	sub := filepath.Join(dir, "pkg")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("import ironclaw\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "lib.go"), []byte("cursor-global-kb reference\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create .git dir (should be skipped)
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte("ironclaw ref\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	findings, err := scanDirectoryWalk(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) < 2 {
		t.Fatalf("expected at least 2 findings, got %d", len(findings))
	}
	for _, f := range findings {
		if strings.Contains(f.File, ".git") {
			t.Fatalf("should not scan .git directory, found: %+v", f)
		}
	}
}

func TestWriteRebrandHuman_NoFindings(t *testing.T) {
	var buf strings.Builder
	writeRebrandHuman(&buf, nil)
	if !strings.Contains(buf.String(), "No legacy terms found") {
		t.Fatalf("expected clean message, got: %q", buf.String())
	}
}

func TestWriteRebrandHuman_WithFindings(t *testing.T) {
	findings := []rebrandFinding{
		{File: "main.go", Line: 5, Category: catBrandName, Match: "ironclaw", Replacement: "helixon"},
	}
	var buf strings.Builder
	writeRebrandHuman(&buf, findings)
	out := buf.String()
	if !strings.Contains(out, "main.go:5") {
		t.Fatalf("expected file:line, got: %q", out)
	}
	if !strings.Contains(out, "brand-name") {
		t.Fatalf("expected category, got: %q", out)
	}
}

func TestWriteRebrandJSON(t *testing.T) {
	findings := []rebrandFinding{
		{File: "main.go", Line: 3, Category: catGoModule, Match: "github.com/nfsarch33/ironclaw-mcp", Replacement: "github.com/nfsarch33/helixon-mcp"},
	}
	var buf strings.Builder
	if err := writeRebrandJSON(&buf, findings); err != nil {
		t.Fatal(err)
	}

	var parsed struct {
		Count    int              `json:"count"`
		Findings []rebrandFinding `json:"findings"`
	}
	if err := json.Unmarshal([]byte(buf.String()), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Count != 1 {
		t.Fatalf("count: got %d want 1", parsed.Count)
	}
	if parsed.Findings[0].Category != catGoModule {
		t.Fatalf("category: got %q want %q", parsed.Findings[0].Category, catGoModule)
	}
}

// TestRebrandScan_DocRepoFlag: --doc-repo skips brand-name, tool-name, and
// deprecated-name categories; only go-module-path, k8s-label, docker-image,
// and env-var categories are enforced.
func TestRebrandScan_DocRepoFlag(t *testing.T) {
	// brand-name/tool-name/deprecated-name: must be suppressed with --doc-repo.
	brandOnlyContent := "IronClaw is the old brand name.\ncursor-global-kb was renamed.\ncylrl is deprecated.\n"
	// go-module-path: must still fire even with --doc-repo.
	moduleContent := "module github.com/nfsarch33/ironclaw-mcp\n"

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "history.md"), []byte(brandOnlyContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(moduleContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Without --doc-repo: history.md should produce findings.
	allFindings, err := scanFile(filepath.Join(dir, "history.md"), "history.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(allFindings) == 0 {
		t.Fatal("brand/deprecated terms must fire without doc-repo flag")
	}

	// With --doc-repo: history.md must produce 0 findings.
	docRepoFindings, err := scanFileDocRepo(filepath.Join(dir, "history.md"), "history.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(docRepoFindings) != 0 {
		t.Fatalf("brand/tool/deprecated categories must be skipped in doc-repo mode, got %d: %+v", len(docRepoFindings), docRepoFindings)
	}

	// go.mod with a go-module-path term must still fire in doc-repo mode.
	modFindings, err := scanFileDocRepo(filepath.Join(dir, "go.mod"), "go.mod")
	if err != nil {
		t.Fatal(err)
	}
	if len(modFindings) == 0 {
		t.Fatal("go-module-path must still fire in doc-repo mode")
	}
}

// TestDocRepoCategoriesEnforced ensures exactly the right categories are still
// enforced when --doc-repo is set.
func TestDocRepoCategoriesEnforced(t *testing.T) {
	dir := t.TempDir()

	cases := []struct {
		name     string
		content  string
		category rebrandCategory
		wantHit  bool
	}{
		{"brand-name", "IronClaw here\n", catBrandName, false},
		{"tool-name", "cursor-global-kb here\n", catToolName, false},
		{"deprecated-name", "evomap here\n", catDeprecated, false},
		{"env-var", "IRONCLAW_TOKEN=x\n", catEnvVar, true},
		{"k8s-label", "namespace: ironclaw-system\n", catK8sLabel, true},
		{"docker-image", "image: ironclaw/agent\n", catDockerImage, true},
		{"go-module-path", "module github.com/nfsarch33/ironclaw-mcp\n", catGoModule, true},
	}

	for _, tc := range cases {
		t.Run(string(tc.category), func(t *testing.T) {
			path := filepath.Join(dir, tc.name+".txt")
			if err := os.WriteFile(path, []byte(tc.content), 0o644); err != nil {
				t.Fatal(err)
			}
			findings, err := scanFileDocRepo(path, tc.name+".txt")
			if err != nil {
				t.Fatal(err)
			}
			if tc.wantHit && len(findings) == 0 {
				t.Fatalf("category %s must still fire in doc-repo mode but got 0 findings", tc.category)
			}
			if !tc.wantHit && len(findings) > 0 {
				t.Fatalf("category %s must be suppressed in doc-repo mode but got %d findings: %+v", tc.category, len(findings), findings)
			}
		})
	}
}

// TestScanDirectory_LoadsAllowlistYAML verifies that scanDirectory automatically
// loads .rebrand-allowlist.yaml from the root and suppresses matching findings.
func TestScanDirectory_LoadsAllowlistYAML(t *testing.T) {
	dir := t.TempDir()

	// A file with a legacy term that IS in rebrandRules (ironclaw -> brand-name).
	if err := os.WriteFile(filepath.Join(dir, "history.md"), []byte("ironclaw was the old name\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Confirm the finding fires WITHOUT allowlist.
	findingsBefore, err := scanDirectoryWalk(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(findingsBefore) == 0 {
		t.Fatal("ironclaw must fire without allowlist")
	}

	// Allowlist that suppresses ironclaw in *.md files.
	allowlistContent := "entries:\n  - file: \"*.md\"\n    term: \"ironclaw\"\n"
	if err := os.WriteFile(filepath.Join(dir, ".rebrand-allowlist.yaml"), []byte(allowlistContent), 0o644); err != nil {
		t.Fatal(err)
	}

	findings, err := scanDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	// .rebrand-allowlist.yaml itself will also be scanned -- filter it out.
	var nonAllowlistFindings []rebrandFinding
	for _, f := range findings {
		if !strings.HasSuffix(f.File, ".rebrand-allowlist.yaml") {
			nonAllowlistFindings = append(nonAllowlistFindings, f)
		}
	}
	if len(nonAllowlistFindings) != 0 {
		t.Fatalf("allowlist should suppress ironclaw in .md; got %d findings: %+v", len(nonAllowlistFindings), nonAllowlistFindings)
	}
}

// TestScanDirectory_AllowlistMissingOK verifies that the absence of
// .rebrand-allowlist.yaml does not cause an error -- it is optional.
func TestScanDirectory_AllowlistMissingOK(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "clean.go"), []byte("package clean\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// No .rebrand-allowlist.yaml -- must not error.
	if _, err := scanDirectory(dir); err != nil {
		t.Fatalf("missing allowlist must not cause error: %v", err)
	}
}

// TestRebrandStatusRunxState_PendingWhenDiffPresent verifies that
// runxAliasMigrationStatus returns "PENDING" when the diff file exists.
func TestRebrandStatusRunxState_PendingWhenDiffPresent(t *testing.T) {
	dir := t.TempDir()
	diffPath := filepath.Join(dir, "runx-override-remote.yaml.diff")
	if err := os.WriteFile(diffPath, []byte("# diff\n+ override_remote: helixon\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	state := runxAliasMigrationStatus(diffPath)
	if state != "PENDING" {
		t.Errorf("want PENDING, got %q", state)
	}
}

// TestRebrandStatusRunxState_DoneWhenDiffAbsent verifies that
// runxAliasMigrationStatus returns "DONE" when the diff file is absent
// (operator has applied and removed it).
func TestRebrandStatusRunxState_DoneWhenDiffAbsent(t *testing.T) {
	state := runxAliasMigrationStatus("/nonexistent/path/runx-override-remote.yaml.diff")
	if state != "DONE" {
		t.Errorf("want DONE, got %q", state)
	}
}

// TestRebrandStatusCmd_EmitsStructuredSummary verifies that
// "cursor-tools rebrand status" outputs a summary with done/pending counters.
func TestRebrandStatusCmd_EmitsStructuredSummary(t *testing.T) {
	dir := t.TempDir()
	// Write a minimal rebranding SOP fixture with a markdown table.
	sopContent := `# Helixon Rebranding

## Section 2.1 GitHub Repository Renames

| Repository | Status |
|---|---|
| cursor-tools | DONE |
| ironclaw-ops | DONE |

## Section 2.2 Go Module Path Migrations

| Module | Status |
|---|---|
| ironclaw-mcp | DONE |
| ai-agent-business-stack/go | PENDING |
`
	sopPath := filepath.Join(dir, "helixon-rebranding-coordination.md")
	if err := os.WriteFile(sopPath, []byte(sopContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := newRebrandStatusCmd(sopPath)
	var out strings.Builder
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("rebrand status: %v", err)
	}

	result := out.String()
	// Must include done and pending counts.
	if !strings.Contains(result, "DONE") {
		t.Errorf("output must reference DONE status: %q", result)
	}
	if !strings.Contains(result, "PENDING") {
		t.Errorf("output must reference PENDING status: %q", result)
	}
}

// TestRebrandStatusCmd_ParsesStatusColumn verifies that the status parser
// correctly distinguishes DONE, PENDING, and DEFERRED entries.
func TestRebrandStatusCmd_ParsesStatusColumn(t *testing.T) {
	dir := t.TempDir()
	sopContent := `## Section 2.1

| Item | Status |
|---|---|
| alpha | DONE |
| beta | PENDING |
| gamma | DEFERRED |
| delta | DONE |
`
	sopPath := filepath.Join(dir, "sop.md")
	if err := os.WriteFile(sopPath, []byte(sopContent), 0o644); err != nil {
		t.Fatal(err)
	}

	summary, err := parseRebrandSOP(sopPath)
	if err != nil {
		t.Fatalf("parseRebrandSOP: %v", err)
	}
	if summary.Done != 2 {
		t.Errorf("want Done=2, got %d", summary.Done)
	}
	if summary.Pending != 1 {
		t.Errorf("want Pending=1, got %d", summary.Pending)
	}
	if summary.Deferred != 1 {
		t.Errorf("want Deferred=1, got %d", summary.Deferred)
	}
}

func TestRebrandRules_ReplacementMap(t *testing.T) {
	wantMappings := map[string]string{
		"ironclaw":                          "helixon",
		"IronClaw":                          "Helixon",
		"IRONCLAW":                          "HELIXON",
		"cursor-global-kb":                  "helixon-kb",
		"cylrl":                             "helixon",
		"CYLRL":                             "HELIXON",
		"evomap":                            "evospine",
		"EvoMap":                            "EvoSpine",
		"github.com/nfsarch33/ironclaw-mcp": "github.com/nfsarch33/helixon-mcp",
		"github.com/nfsarch33/ironclaw-ops": "github.com/nfsarch33/helixon-ops",
	}
	ruleMap := map[string]string{}
	for _, r := range rebrandRules {
		ruleMap[r.Pattern] = r.Replacement
	}
	for pattern, expected := range wantMappings {
		got, ok := ruleMap[pattern]
		if !ok {
			t.Fatalf("missing rule for pattern %q", pattern)
		}
		if got != expected {
			t.Fatalf("rule %q: got replacement %q want %q", pattern, got, expected)
		}
	}
}
