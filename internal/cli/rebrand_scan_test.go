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
