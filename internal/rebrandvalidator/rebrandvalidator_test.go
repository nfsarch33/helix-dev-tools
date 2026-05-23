package rebrandvalidator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanFileFindsLegacyTerms(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "example.go")
	content := "package main\n\nimport \"helixon/pkg\"\n\nfunc main() {}\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	findings, err := ScanFile(path, DefaultLegacyTerms())
	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Equal(t, 3, findings[0].LineNumber)
	assert.Equal(t, "helixon", findings[0].LegacyTerm)
	assert.Equal(t, path, findings[0].FilePath)
}

func TestScanFileNoFindings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "clean.go")
	require.NoError(t, os.WriteFile(path, []byte("package clean\n\nfunc Hello() {}\n"), 0644))

	findings, err := ScanFile(path, DefaultLegacyTerms())
	require.NoError(t, err)
	assert.Empty(t, findings)
}

func TestScanDirectoryRecursive(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub", "deep")
	require.NoError(t, os.MkdirAll(sub, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(dir, "root.go"), []byte("helixon\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(sub, "nested.go"), []byte("cursor-tools config\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "clean.go"), []byte("nothing here\n"), 0644))

	result, err := ScanDirectory(dir, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, 2, result.FindingCount)
	assert.GreaterOrEqual(t, result.FileCount, 3)
}

func TestScanDirectoryExcludes(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	vendorDir := filepath.Join(dir, "vendor")
	require.NoError(t, os.MkdirAll(gitDir, 0755))
	require.NoError(t, os.MkdirAll(vendorDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("helixon ref\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(vendorDir, "lib.go"), []byte("cursor-tools dep\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("clean code\n"), 0644))

	result, err := ScanDirectory(dir, []string{".git", "vendor", "node_modules"}, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, result.FindingCount)
}

func TestDefaultLegacyTerms(t *testing.T) {
	terms := DefaultLegacyTerms()
	expected := []string{"helixon", "cursor-tools", "cursor-global-kb", "cursor_tools", "cylrl", "Helixon", "HELIXON"}
	for _, e := range expected {
		assert.Contains(t, terms, e, "missing expected term: %s", e)
	}
	assert.Len(t, terms, len(expected))
}

func TestScanFileMultipleTerms(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "multi.txt")
	content := "line one\nhelixon and cursor-tools here\nHELIXON on line 3\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	findings, err := ScanFile(path, DefaultLegacyTerms())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(findings), 3)
}

func TestFindingHasContext(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ctx.go")
	content := "package main\n\nvar name = \"helixon-service\"\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	findings, err := ScanFile(path, DefaultLegacyTerms())
	require.NoError(t, err)
	require.NotEmpty(t, findings)
	assert.Contains(t, findings[0].Context, "helixon-service")
}

func TestScanDirectoryCountsFiles(t *testing.T) {
	dir := t.TempDir()
	for i := range 5 {
		name := filepath.Join(dir, strings.Replace("file_N.txt", "N", string(rune('a'+i)), 1))
		require.NoError(t, os.WriteFile(name, []byte("clean\n"), 0644))
	}

	result, err := ScanDirectory(dir, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, 5, result.FileCount)
}

func TestScanEmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	result, err := ScanDirectory(dir, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, result.FindingCount)
	assert.Equal(t, 0, result.FileCount)
}

func TestScanFileIgnoresBinaryFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "binary.bin")
	data := make([]byte, 256)
	for i := range data {
		data[i] = byte(i)
	}
	require.NoError(t, os.WriteFile(path, data, 0644))

	findings, err := ScanFile(path, DefaultLegacyTerms())
	require.NoError(t, err)
	assert.Empty(t, findings)
}

func TestScanFileLargeFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.txt")

	var b strings.Builder
	for i := range 10000 {
		if i == 5000 {
			b.WriteString("found helixon here\n")
			continue
		}
		b.WriteString("clean line of text content here\n")
	}
	require.NoError(t, os.WriteFile(path, []byte(b.String()), 0644))

	start := time.Now()
	findings, err := ScanFile(path, DefaultLegacyTerms())
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Len(t, findings, 1)
	assert.Less(t, elapsed, time.Second)
}

// TestScanDirectory_WithAllowlist: file+term combination in the allowlist must
// suppress a finding and increment SuppressedCount, not FindingCount.
func TestScanDirectory_WithAllowlist(t *testing.T) {
	dir := t.TempDir()
	// A file with a legacy term that we want to allowlist.
	path := filepath.Join(dir, "legacy-sop.md")
	require.NoError(t, os.WriteFile(path, []byte("This doc references helixon for historical reasons.\n"), 0644))

	// GIVEN: allowlist suppresses helixon in *.md files.
	allowlist := []AllowlistEntry{
		{File: "*.md", Term: "helixon"},
	}

	// WHEN: ScanDirectory called with allowlist.
	result, err := ScanDirectory(dir, nil, allowlist)
	require.NoError(t, err)

	// THEN: finding is suppressed, FindingCount == 0, SuppressedCount == 1.
	assert.Equal(t, 0, result.FindingCount, "suppressed finding must not count as a finding")
	assert.Equal(t, 1, result.SuppressedCount, "SuppressedCount must reflect suppressed findings")
	assert.Empty(t, result.Findings, "Findings slice must be empty when all are suppressed")
}

// TestScanDirectory_AllowlistDoesNotSuppressOtherTerms ensures the allowlist is
// scoped: a file+term allowlist entry only suppresses that specific term in that
// file pattern, not other terms.
func TestScanDirectory_AllowlistDoesNotSuppressOtherTerms(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")
	require.NoError(t, os.WriteFile(path, []byte("helixon and cursor-tools both appear here.\n"), 0644))

	// Only suppress helixon in .md files; cursor-tools is NOT suppressed.
	allowlist := []AllowlistEntry{
		{File: "*.md", Term: "helixon"},
	}

	result, err := ScanDirectory(dir, nil, allowlist)
	require.NoError(t, err)

	// cursor-tools must still be reported; helixon must be suppressed.
	assert.Equal(t, 1, result.FindingCount)
	assert.Equal(t, 1, result.SuppressedCount)
}

// TestScanDirectory_EmptyAllowlist verifies backward-compatible behaviour:
// nil allowlist produces the same result as before.
func TestScanDirectory_EmptyAllowlist(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("helixon ref\n"), 0644))

	result, err := ScanDirectory(dir, nil, nil)
	require.NoError(t, err)

	assert.Equal(t, 1, result.FindingCount)
	assert.Equal(t, 0, result.SuppressedCount)
}

// TestLoadAllowlistYAML_ParsesEntries verifies well-formed YAML is parsed correctly.
func TestLoadAllowlistYAML_ParsesEntries(t *testing.T) {
	dir := t.TempDir()
	content := `entries:
  - file: "*.md"
    term: "cursor-tools"
  - file: "Makefile"
    term: "cursor-tools"
`
	path := filepath.Join(dir, ".rebrand-allowlist.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	entries, err := LoadAllowlistYAML(path)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "*.md", entries[0].File)
	assert.Equal(t, "cursor-tools", entries[0].Term)
	assert.Equal(t, "Makefile", entries[1].File)
	assert.Equal(t, "cursor-tools", entries[1].Term)
}

// TestLoadAllowlistYAML_MissingFileIsOK verifies that a missing allowlist file
// returns an empty slice without error (opt-in, not required).
func TestLoadAllowlistYAML_MissingFileIsOK(t *testing.T) {
	entries, err := LoadAllowlistYAML("/nonexistent/.rebrand-allowlist.yaml")
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestScanResultJSON(t *testing.T) {
	result := ScanResult{
		Findings: []Finding{
			{FilePath: "test.go", LineNumber: 1, LegacyTerm: "helixon", Context: "helixon ref", Line: "helixon ref"},
		},
		FileCount:    1,
		FindingCount: 1,
	}

	data, err := json.Marshal(result)
	require.NoError(t, err)

	var decoded ScanResult
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, result.FindingCount, decoded.FindingCount)
	assert.Len(t, decoded.Findings, 1)
}

// TestScanResult_HasFindings: ScanResult.HasFindings() returns true when
// FindingCount > 0, false when FindingCount == 0.
func TestScanResult_HasFindings(t *testing.T) {
	tests := []struct {
		name         string
		findingCount int
		want         bool
	}{
		{"zero findings", 0, false},
		{"one finding", 1, true},
		{"multiple findings", 5, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := ScanResult{FindingCount: tc.findingCount}
			assert.Equal(t, tc.want, r.HasFindings())
		})
	}
}

// TestScanResult_HasFindings_SuppressedDoNotCount: suppressed findings do not
// make HasFindings() true -- only FindingCount matters.
func TestScanResult_HasFindings_SuppressedDoNotCount(t *testing.T) {
	r := ScanResult{FindingCount: 0, SuppressedCount: 5}
	assert.False(t, r.HasFindings(), "suppressed-only result must not be HasFindings")
}
