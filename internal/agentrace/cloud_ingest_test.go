package agentrace

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCloudIngest_AcceptsNDJSON(t *testing.T) {
	dir := t.TempDir()
	handler, err := NewCloudIngestHandler(dir, nil)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	body := fmt.Sprintf(`{"ts":"%s","event":"test1"}
{"ts":"%s","event":"test2"}
{"ts":"%s","event":"test3"}
`, now, now, now)

	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-ndjson")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if handler.Count() != 3 {
		t.Errorf("expected 3 ingested events, got %d", handler.Count())
	}
}

func TestCloudIngest_RejectsGet(t *testing.T) {
	dir := t.TempDir()
	handler, err := NewCloudIngestHandler(dir, nil)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/ingest", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestCloudIngest_HandlesInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	handler, err := NewCloudIngestHandler(dir, nil)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	body := `{"ts":"2026-01-01T00:00:00Z","event":"valid"}
not json
{"ts":"2026-01-01T00:00:00Z","event":"also valid"}
`

	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-ndjson")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if handler.Count() != 2 {
		t.Errorf("expected 2 accepted events (1 invalid), got %d", handler.Count())
	}
}

func TestCloudIngest_1000Events(t *testing.T) {
	dir := t.TempDir()
	handler, err := NewCloudIngestHandler(dir, nil)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	var b strings.Builder
	now := time.Now().UTC()
	for i := 0; i < 1000; i++ {
		ts := now.Add(time.Duration(i) * time.Millisecond).Format(time.RFC3339Nano)
		fmt.Fprintf(&b, `{"ts":"%s","event":"soak-%d","stream":"test"}`, ts, i)
		b.WriteByte('\n')
	}

	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(b.String()))
	req.Header.Set("Content-Type", "application/x-ndjson")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if handler.Count() != 1000 {
		t.Errorf("expected 1000 ingested events, got %d", handler.Count())
	}
}
