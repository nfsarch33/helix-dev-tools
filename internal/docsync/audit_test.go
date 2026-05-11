package docsync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditRepoCatchesVersionAndADRDrift(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "VERSION", "6.0.0\n")
	writeFile(t, root, "README.md", "# Demo\n\nCurrent release v5.0.0.\n")
	writeFile(t, root, "CHANGELOG.md", "## 6.0.0\n")
	writeFile(t, root, "package.json", `{"name":"demo","version":"6.0.0"}`)
	writeFile(t, root, "docs/adr/adr-001-demo.md", "# ADR 001\n")
	writeFile(t, root, "docs/adr/README.md", "# ADR Index\n")
	writeFile(t, root, "LICENSE", "MIT\n")
	writeFile(t, root, "CONTRIBUTING.md", "# Contributing\n")

	report, err := AuditRepo(root)
	if err != nil {
		t.Fatalf("AuditRepo: %v", err)
	}

	if report.OK() {
		t.Fatalf("expected docs drift findings")
	}
	if !hasFinding(report, "README_VERSION") {
		t.Fatalf("expected README_VERSION finding, got %#v", report.Findings)
	}
	if !hasFinding(report, "ADR_INDEX") {
		t.Fatalf("expected ADR_INDEX finding, got %#v", report.Findings)
	}
}

func TestFixRepoRegeneratesADRIndex(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "README.md", "# Demo\n\nCurrent release: **v5.0.0**.\n")
	writeFile(t, root, "VERSION", "6.0.0\n")
	writeFile(t, root, "CHANGELOG.md", "## 6.0.0\n")
	writeFile(t, root, "api/openapi.yaml", "openapi: 3.1.0\ninfo:\n  title: Demo\n  version: 5.9.0\npaths: {}\n")
	writeFile(t, root, "docs/adr/adr-002-second.md", "# ADR 002\n")
	writeFile(t, root, "docs/adr/adr-001-first.md", "# ADR 001\n")
	writeFile(t, root, "LICENSE", "MIT\n")
	writeFile(t, root, "CONTRIBUTING.md", "# Contributing\n")

	report, err := FixRepo(root)
	if err != nil {
		t.Fatalf("FixRepo: %v", err)
	}
	for _, want := range []string{"README.md", filepath.Join("api", "openapi.yaml"), filepath.Join("docs", "adr", "README.md")} {
		if !contains(report.Fixed, want) {
			t.Fatalf("fixed = %#v, missing %s", report.Fixed, want)
		}
	}

	readmeRaw, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	if !strings.Contains(string(readmeRaw), "Current release: **v6.0.0**.") {
		t.Fatalf("README version not fixed:\n%s", readmeRaw)
	}

	openAPIRaw, err := os.ReadFile(filepath.Join(root, "api", "openapi.yaml"))
	if err != nil {
		t.Fatalf("read openapi: %v", err)
	}
	if !strings.Contains(string(openAPIRaw), "  version: 6.0.0") {
		t.Fatalf("openapi version not fixed:\n%s", openAPIRaw)
	}

	raw, err := os.ReadFile(filepath.Join(root, "docs", "adr", "README.md"))
	if err != nil {
		t.Fatalf("read ADR index: %v", err)
	}
	got := string(raw)
	first := strings.Index(got, "adr-001-first.md")
	second := strings.Index(got, "adr-002-second.md")
	if first == -1 || second == -1 || first > second {
		t.Fatalf("ADR index not sorted or missing entries:\n%s", got)
	}
}

func TestAuditRepoDoesNotRequireLicenseForDocsOnlyRepo(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "README.md", "# Knowledge Base\n")

	report, err := AuditRepo(root)
	if err != nil {
		t.Fatalf("AuditRepo: %v", err)
	}
	if !report.OK() {
		t.Fatalf("docs-only repo should not require code repo files, got %#v", report.Findings)
	}
}

func TestAuditRepoCanRequirePublicRepoFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "README.md", "# Public Tool\n\nCurrent release v1.0.0.\n")
	writeFile(t, root, "VERSION", "1.0.0\n")
	writeFile(t, root, "CHANGELOG.md", "## 1.0.0\n")

	report, err := AuditRepoWithOptions(root, Options{RequirePublicFiles: true})
	if err != nil {
		t.Fatalf("AuditRepoWithOptions: %v", err)
	}
	if !hasFinding(report, "REQUIRED_FILE") {
		t.Fatalf("expected public repo file finding, got %#v", report.Findings)
	}
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func hasFinding(report Report, code string) bool {
	for _, finding := range report.Findings {
		if finding.Code == code {
			return true
		}
	}
	return false
}
