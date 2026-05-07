package mem0

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeSearcher implements Searcher for test injection.
type fakeSearcher struct {
	results func(query string) []SearchResult
}

func (f *fakeSearcher) Search(ctx context.Context, query string) ([]SearchResult, error) {
	if f.results != nil {
		return f.results(query), nil
	}
	return nil, nil
}

func TestReadFlip_TopThreeOverlapAtLeast80Pct(t *testing.T) {
	sharedResults := []SearchResult{
		{ID: "m-1", Text: "cursor rules v299", Score: 0.98},
		{ID: "m-2", Text: "evoloop daemon", Score: 0.95},
		{ID: "m-3", Text: "mem0 canary", Score: 0.91},
		{ID: "m-4", Text: "workspace doctor", Score: 0.85},
	}

	oss := &fakeSearcher{results: func(_ string) []SearchResult {
		return sharedResults
	}}
	managed := &fakeSearcher{results: func(_ string) []SearchResult {
		return sharedResults
	}}

	logDir := t.TempDir()
	logPath := filepath.Join(logDir, "readflip.ndjson")

	rf := &ReadFlip{
		OSS:     oss,
		Managed: managed,
		FlipPct: 100,
		LogPath: logPath,
		Queries: []string{"cursor rules", "evoloop status", "mem0 health"},
		Timeout: 5 * time.Second,
	}

	report, err := rf.Run(context.Background())
	if err != nil {
		t.Fatalf("ReadFlip.Run: %v", err)
	}

	if report.TotalQueries != 3 {
		t.Errorf("total queries: got %d, want 3", report.TotalQueries)
	}
	if report.FlippedToOSS != 3 {
		t.Errorf("flipped to OSS: got %d, want 3", report.FlippedToOSS)
	}
	if report.AvgOverlap < 0.80 {
		t.Errorf("avg top-3 overlap: got %.2f, want >= 0.80", report.AvgOverlap)
	}

	// Verify NDJSON log file was written with one entry per query.
	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(logData)), "\n")
	if len(lines) != 3 {
		t.Fatalf("log lines: got %d, want 3", len(lines))
	}
	for i, line := range lines {
		var entry OverlapLogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("line %d: unmarshal: %v", i, err)
		}
		if entry.Overlap < 0.80 {
			t.Errorf("line %d: overlap %.2f < 0.80", i, entry.Overlap)
		}
	}
}

func TestReadFlip_ZeroFlipPctUsesManagedOnly(t *testing.T) {
	ossHit := false
	oss := &fakeSearcher{results: func(_ string) []SearchResult {
		ossHit = true
		return nil
	}}
	managed := &fakeSearcher{results: func(_ string) []SearchResult {
		return []SearchResult{{ID: "m-1", Text: "ok", Score: 0.9}}
	}}

	rf := &ReadFlip{
		OSS:     oss,
		Managed: managed,
		FlipPct: 0,
		LogPath: filepath.Join(t.TempDir(), "log.ndjson"),
		Queries: []string{"test"},
		Timeout: time.Second,
	}

	report, err := rf.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if ossHit {
		t.Error("OSS should not be called when FlipPct=0")
	}
	if report.FlippedToOSS != 0 {
		t.Errorf("flipped: got %d, want 0", report.FlippedToOSS)
	}
}

func TestReadFlip_PartialOverlapRecorded(t *testing.T) {
	oss := &fakeSearcher{results: func(_ string) []SearchResult {
		return []SearchResult{
			{ID: "a", Text: "alpha", Score: 0.99},
			{ID: "b", Text: "beta", Score: 0.95},
			{ID: "c", Text: "gamma", Score: 0.90},
		}
	}}
	managed := &fakeSearcher{results: func(_ string) []SearchResult {
		return []SearchResult{
			{ID: "a", Text: "alpha", Score: 0.99},
			{ID: "x", Text: "other", Score: 0.92},
			{ID: "c", Text: "gamma", Score: 0.88},
		}
	}}

	logPath := filepath.Join(t.TempDir(), "log.ndjson")
	rf := &ReadFlip{
		OSS:     oss,
		Managed: managed,
		FlipPct: 100,
		LogPath: logPath,
		Queries: []string{"test"},
		Timeout: time.Second,
	}

	report, err := rf.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// 2 of 3 IDs overlap (a, c) => overlap = 2/3 ≈ 0.667
	wantOverlap := 2.0 / 3.0
	if report.AvgOverlap < wantOverlap-0.01 || report.AvgOverlap > wantOverlap+0.01 {
		t.Errorf("overlap: got %.4f, want ~%.4f", report.AvgOverlap, wantOverlap)
	}
}

func TestReadFlip_EmptyQueriesReturnsError(t *testing.T) {
	rf := &ReadFlip{
		OSS:     &fakeSearcher{},
		Managed: &fakeSearcher{},
		FlipPct: 50,
		LogPath: filepath.Join(t.TempDir(), "log.ndjson"),
		Timeout: time.Second,
	}
	_, err := rf.Run(context.Background())
	if err == nil {
		t.Fatal("expected error for empty queries")
	}
}
