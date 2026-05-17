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
	content := "package main\n\nimport \"ironclaw/pkg\"\n\nfunc main() {}\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	findings, err := ScanFile(path, DefaultLegacyTerms())
	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Equal(t, 3, findings[0].LineNumber)
	assert.Equal(t, "ironclaw", findings[0].LegacyTerm)
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

	require.NoError(t, os.WriteFile(filepath.Join(dir, "root.go"), []byte("ironclaw\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(sub, "nested.go"), []byte("cursor-tools config\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "clean.go"), []byte("nothing here\n"), 0644))

	result, err := ScanDirectory(dir, nil)
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

	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ironclaw ref\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(vendorDir, "lib.go"), []byte("cursor-tools dep\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("clean code\n"), 0644))

	result, err := ScanDirectory(dir, []string{".git", "vendor", "node_modules"})
	require.NoError(t, err)
	assert.Equal(t, 0, result.FindingCount)
}

func TestDefaultLegacyTerms(t *testing.T) {
	terms := DefaultLegacyTerms()
	expected := []string{"ironclaw", "cursor-tools", "cursor-global-kb", "cursor_tools", "cylrl", "IronClaw", "IRONCLAW"}
	for _, e := range expected {
		assert.Contains(t, terms, e, "missing expected term: %s", e)
	}
	assert.Len(t, terms, len(expected))
}

func TestScanFileMultipleTerms(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "multi.txt")
	content := "line one\nironclaw and cursor-tools here\nIRONCLAW on line 3\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	findings, err := ScanFile(path, DefaultLegacyTerms())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(findings), 3)
}

func TestFindingHasContext(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ctx.go")
	content := "package main\n\nvar name = \"ironclaw-service\"\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	findings, err := ScanFile(path, DefaultLegacyTerms())
	require.NoError(t, err)
	require.NotEmpty(t, findings)
	assert.Contains(t, findings[0].Context, "ironclaw-service")
}

func TestScanDirectoryCountsFiles(t *testing.T) {
	dir := t.TempDir()
	for i := range 5 {
		name := filepath.Join(dir, strings.Replace("file_N.txt", "N", string(rune('a'+i)), 1))
		require.NoError(t, os.WriteFile(name, []byte("clean\n"), 0644))
	}

	result, err := ScanDirectory(dir, nil)
	require.NoError(t, err)
	assert.Equal(t, 5, result.FileCount)
}

func TestScanEmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	result, err := ScanDirectory(dir, nil)
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
			b.WriteString("found ironclaw here\n")
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

func TestScanResultJSON(t *testing.T) {
	result := ScanResult{
		Findings: []Finding{
			{FilePath: "test.go", LineNumber: 1, LegacyTerm: "ironclaw", Context: "ironclaw ref", Line: "ironclaw ref"},
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
