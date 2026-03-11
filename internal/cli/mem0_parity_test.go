package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecodeMem0ListResponseHandlesWrappedEmptyResults(t *testing.T) {
	data := []byte(`{"results":[],"total":0}`)
	items, total, err := decodeMem0ListResponse(data)
	if err != nil {
		t.Fatalf("decodeMem0ListResponse() error = %v", err)
	}
	if len(items) != 0 || total != 0 {
		t.Fatalf("decodeMem0ListResponse() = %d items, total=%d", len(items), total)
	}
}

func TestCompareParityReportsExactContentAndMissing(t *testing.T) {
	manifest := []parityManifestEntry{
		newManifestEntry("pattern", "/Users/test/Code/global-kb/learnings/PATTERNS.md", "pat-001", "Use lockfiles", "Use lockfiles"),
		newManifestEntry("learning", "/Users/test/Code/global-kb/learnings/LEARNINGS.md", "[2026-03-05] Category: best_practice", "Absolute git paths", "Shell commands in hooks must use absolute git paths"),
		newManifestEntry("error", "/Users/test/Code/global-kb/learnings/ERRORS.md", "[2026-03-06] Category: security", "Missing item", "This one is absent"),
	}
	remote := []mem0RemoteMemory{
		{
			ID:     "exact-1",
			Memory: "Use lockfiles for concurrent hook runs",
			Metadata: map[string]any{
				"source_path": "learnings/PATTERNS.md",
				"source_id":   "pat-001",
			},
		},
		{
			ID:     "content-1",
			Memory: "Shell commands in hooks must use absolute git paths to avoid guard interference",
		},
	}

	report := compareParity(manifest, remote, mem0AuditConfig{UserID: "u", AppID: "a"})
	if len(report.ExactMatches) != 1 {
		t.Fatalf("expected 1 exact match, got %d", len(report.ExactMatches))
	}
	if len(report.ContentMatches) != 1 {
		t.Fatalf("expected 1 content match, got %d", len(report.ContentMatches))
	}
	if len(report.Missing) != 1 {
		t.Fatalf("expected 1 missing entry, got %d", len(report.Missing))
	}
	if report.Proven() {
		t.Fatal("expected parity not to be proven")
	}
}

func TestBuildPatternManifestParsesPatternTable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "PATTERNS.md")
	content := strings.Join([]string{
		"# Global Semantic Patterns",
		"",
		"| ID | Pattern | Confidence | Applications | Category | Created | Projects |",
		"|----|---------|------------|--------------|----------|---------|---------|",
		"| pat-001 | Always do X | 1.0 | 5+ | test | 2026-03-05 | global |",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	entries, err := buildPatternManifest(path)
	if err != nil {
		t.Fatalf("buildPatternManifest() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].SourceID != "pat-001" || entries[0].Title != "Always do X" {
		t.Fatalf("unexpected entry: %+v", entries[0])
	}
}
