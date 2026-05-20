package rebrander

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestNewScanner(t *testing.T) {
	rules := []Rule{{Old: "helixon", New: "ironclaw"}}
	s := NewScanner(rules)
	if len(s.Rules) != 1 {
		t.Errorf("rules: %d", len(s.Rules))
	}
}

func TestScanFindsMatches(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "config.yaml", "host: helixon-mcp\nport: 8080\n")
	writeFile(t, dir, "readme.md", "This uses ironclaw (not helixon).\n")

	s := NewScanner([]Rule{{Old: "helixon", New: "ironclaw"}})
	findings, err := s.Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 2 {
		t.Errorf("expected 2 findings, got %d", len(findings))
	}
}

func TestScanCountsOccurrences(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "test.go", "helixon helixon helixon\n")

	s := NewScanner([]Rule{{Old: "helixon", New: "ironclaw"}})
	findings, err := s.Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if findings[0].Count != 3 {
		t.Errorf("count: %d", findings[0].Count)
	}
}

func TestScanSkipsBinary(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "binary.exe", "\x00\x01helixon\x00\x02")
	writeFile(t, dir, "text.go", "helixon\n")

	s := NewScanner([]Rule{{Old: "helixon", New: "ironclaw"}})
	findings, err := s.Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Errorf("expected 1 finding (skipping binary), got %d", len(findings))
	}
}

func TestApply(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "config.yaml", "host: helixon-mcp\nalias: helixon\n")

	s := NewScanner([]Rule{{Old: "helixon", New: "ironclaw"}})
	count, err := s.Apply(path, s.Rules[0])
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("replacements: %d", count)
	}

	data, _ := os.ReadFile(path)
	content := string(data)
	if content != "host: ironclaw-mcp\nalias: ironclaw\n" {
		t.Errorf("content after apply: %q", content)
	}
}

func TestApplyNoMatch(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "clean.go", "all good here\n")

	s := NewScanner([]Rule{{Old: "helixon", New: "ironclaw"}})
	count, err := s.Apply(path, s.Rules[0])
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("expected 0 replacements, got %d", count)
	}
}

func TestMultipleRules(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "mixed.go", "helixon and helix-ops both need updating\n")

	s := NewScanner([]Rule{
		{Old: "helixon", New: "ironclaw"},
		{Old: "helix-ops", New: "ironclaw-ops"},
	})
	findings, err := s.Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 2 {
		t.Errorf("expected 2 findings for 2 rules, got %d", len(findings))
	}
}

func TestScanSubdirectories(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0755)
	writeFile(t, sub, "deep.go", "helixon inside subdir\n")

	s := NewScanner([]Rule{{Old: "helixon", New: "ironclaw"}})
	findings, err := s.Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Errorf("expected 1 finding in subdir, got %d", len(findings))
	}
}

func TestScanEmptyDir(t *testing.T) {
	dir := t.TempDir()
	s := NewScanner([]Rule{{Old: "helixon", New: "ironclaw"}})
	findings, err := s.Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}
