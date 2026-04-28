package mem0outbox

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestWriter_AppendDurableLines(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(filepath.Join(dir, "pending.jsonl"))
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	defer w.Close()

	for i := 0; i < 3; i++ {
		c := Capsule{
			ID:     "id-" + strconv.Itoa(i),
			AppID:  "cursor-global-kb",
			UserID: "macbook-replica",
			Text:   "outbox capsule " + strconv.Itoa(i),
			Metadata: map[string]string{
				"kind":    "evoloop_cycle",
				"machine": "macbook",
			},
		}
		if err := w.Append(c); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}

	body, err := os.ReadFile(filepath.Join(dir, "pending.jsonl"))
	if err != nil {
		t.Fatalf("read pending: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) != 3 {
		t.Fatalf("want 3 lines, got %d", len(lines))
	}
	for i, line := range lines {
		var got Capsule
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			t.Fatalf("line %d: %v", i, err)
		}
		if got.ID != "id-"+strconv.Itoa(i) {
			t.Errorf("line %d: id=%q", i, got.ID)
		}
		if got.Metadata["machine"] != "macbook" {
			t.Errorf("line %d: machine=%q", i, got.Metadata["machine"])
		}
	}
}

func TestReader_TailDedupsByID(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(filepath.Join(dir, "pending.jsonl"))
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	defer w.Close()

	for i := 0; i < 5; i++ {
		_ = w.Append(Capsule{ID: "dup", Text: "old #" + strconv.Itoa(i)})
	}
	_ = w.Append(Capsule{ID: "uniq-1", Text: "first"})
	_ = w.Append(Capsule{ID: "uniq-2", Text: "second"})

	r := NewReader(filepath.Join(dir, "pending.jsonl"), filepath.Join(dir, "cursor"))
	caps, err := r.Tail(10)
	if err != nil {
		t.Fatalf("Tail: %v", err)
	}
	// Dedupe should keep the most recent occurrence of "dup" and order
	// newest-first.
	if len(caps) != 3 {
		t.Fatalf("want 3 deduped capsules, got %d", len(caps))
	}
	if caps[0].ID != "uniq-2" || caps[1].ID != "uniq-1" || caps[2].ID != "dup" {
		t.Fatalf("order wrong: %v", []string{caps[0].ID, caps[1].ID, caps[2].ID})
	}
	if caps[2].Text != "old #4" {
		t.Errorf("dedupe took wrong line: %q", caps[2].Text)
	}
}

func TestFlusher_DrainsPendingAndAdvancesCursor(t *testing.T) {
	dir := t.TempDir()
	w, _ := NewWriter(filepath.Join(dir, "pending.jsonl"))
	defer w.Close()
	for i := 0; i < 3; i++ {
		_ = w.Append(Capsule{ID: "c-" + strconv.Itoa(i), Text: "t" + strconv.Itoa(i)})
	}

	var got int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&got, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"ok"}`))
	}))
	defer upstream.Close()

	cli := NewMem0Client(upstream.URL, "tok")
	cli.HTTP = upstream.Client()

	flusher := &Flusher{
		PendingPath: filepath.Join(dir, "pending.jsonl"),
		CursorPath:  filepath.Join(dir, "cursor"),
		Client:      cli,
		BatchSize:   10,
	}
	report, err := flusher.Flush(context.Background())
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if report.Flushed != 3 {
		t.Fatalf("want 3 flushed, got %d (skipped=%d retryAfter=%v)", report.Flushed, report.Skipped, report.RetryAfter)
	}
	if got := atomic.LoadInt32(&got); got != 3 {
		t.Fatalf("upstream calls: want 3, got %d", got)
	}
	// Cursor file must reflect the full pending file size.
	cursorBytes, _ := os.ReadFile(filepath.Join(dir, "cursor"))
	pendingInfo, _ := os.Stat(filepath.Join(dir, "pending.jsonl"))
	want := strconv.FormatInt(pendingInfo.Size(), 10)
	if strings.TrimSpace(string(cursorBytes)) != want {
		t.Fatalf("cursor=%q want=%q", string(cursorBytes), want)
	}

	// Subsequent flush with no new entries is a no-op and not an error.
	report2, err := flusher.Flush(context.Background())
	if err != nil {
		t.Fatalf("second Flush: %v", err)
	}
	if report2.Flushed != 0 {
		t.Fatalf("second flush: want 0 flushed, got %d", report2.Flushed)
	}
}

func TestFlusher_429RespectsRetryAfterAndDoesNotAdvance(t *testing.T) {
	dir := t.TempDir()
	w, _ := NewWriter(filepath.Join(dir, "pending.jsonl"))
	defer w.Close()
	for i := 0; i < 4; i++ {
		_ = w.Append(Capsule{ID: "c-" + strconv.Itoa(i), Text: "t"})
	}

	var firstCallSucceeded int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// First two calls succeed, then we 429.
		if atomic.AddInt32(&firstCallSucceeded, 1) <= 2 {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"ok"}`))
			return
		}
		w.Header().Set("Retry-After", "13")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"detail":"rate limited"}`))
	}))
	defer upstream.Close()

	cli := NewMem0Client(upstream.URL, "tok")
	cli.HTTP = upstream.Client()
	flusher := &Flusher{
		PendingPath: filepath.Join(dir, "pending.jsonl"),
		CursorPath:  filepath.Join(dir, "cursor"),
		Client:      cli,
		BatchSize:   10,
	}
	report, err := flusher.Flush(context.Background())
	if err == nil {
		t.Fatalf("want 429 error, got nil")
	}
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("want ErrRateLimited, got %v", err)
	}
	if report.Flushed != 2 {
		t.Fatalf("want 2 flushed before 429, got %d", report.Flushed)
	}
	if report.RetryAfter != 13*time.Second {
		t.Fatalf("want retry-after 13s, got %v", report.RetryAfter)
	}

	// Cursor must point at the byte offset right after the second NDJSON
	// line (so the next flush picks up at line 3).
	pending, _ := os.ReadFile(filepath.Join(dir, "pending.jsonl"))
	lines := strings.Split(string(pending), "\n")
	wantOffset := len(lines[0]) + 1 + len(lines[1]) + 1 // bytes of first two lines + their LFs
	cursorBytes, _ := os.ReadFile(filepath.Join(dir, "cursor"))
	if strings.TrimSpace(string(cursorBytes)) != strconv.Itoa(wantOffset) {
		t.Fatalf("cursor=%q want=%d", string(cursorBytes), wantOffset)
	}

	// Now allow upstream to succeed and resume; remaining 2 should drain.
	atomic.StoreInt32(&firstCallSucceeded, 0)
	upstream2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"ok"}`))
	}))
	defer upstream2.Close()
	cli2 := NewMem0Client(upstream2.URL, "tok")
	cli2.HTTP = upstream2.Client()
	flusher.Client = cli2
	report2, err := flusher.Flush(context.Background())
	if err != nil {
		t.Fatalf("resume Flush: %v", err)
	}
	if report2.Flushed != 2 {
		t.Fatalf("resume: want 2 flushed, got %d", report2.Flushed)
	}
}

func TestMem0Client_PostsCapsuleEnvelope(t *testing.T) {
	var captured []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("want POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/v1/memories/") {
			t.Errorf("want /v1/memories/, got %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Token tok" {
			t.Errorf("auth=%q", got)
		}
		captured, _ = readBody(r)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"new"}`))
	}))
	defer upstream.Close()

	cli := NewMem0Client(upstream.URL, "tok")
	cli.HTTP = upstream.Client()
	c := Capsule{
		ID:     "abc",
		AppID:  "cursor-global-kb",
		UserID: "macbook",
		Text:   "ping",
		Metadata: map[string]string{
			"kind":    "evoloop_cycle",
			"machine": "macbook",
		},
	}
	if err := cli.Push(context.Background(), c); err != nil {
		t.Fatalf("Push: %v", err)
	}
	var payload struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		AppID    string            `json:"app_id"`
		UserID   string            `json:"user_id"`
		Metadata map[string]string `json:"metadata"`
	}
	if err := json.Unmarshal(captured, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.AppID != "cursor-global-kb" || payload.UserID != "macbook" {
		t.Fatalf("envelope app_id/user_id wrong: %+v", payload)
	}
	if payload.Metadata["kind"] != "evoloop_cycle" {
		t.Fatalf("metadata.kind=%q", payload.Metadata["kind"])
	}
	if len(payload.Messages) != 1 || payload.Messages[0].Content != "ping" {
		t.Fatalf("messages wrong: %+v", payload.Messages)
	}
}

// readBody is a tiny helper used only in tests to avoid importing io
// twice across the file groupings.
func readBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 1024)
	for {
		n, err := r.Body.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	return buf, nil
}

func TestFlusher_MissingPendingIsNoOp(t *testing.T) {
	dir := t.TempDir()
	flusher := &Flusher{
		PendingPath: filepath.Join(dir, "missing.jsonl"),
		CursorPath:  filepath.Join(dir, "cursor"),
		Client:      &fakePusher{},
	}
	report, err := flusher.Flush(context.Background())
	if err != nil {
		t.Fatalf("missing pending: %v", err)
	}
	if report.Flushed != 0 || report.Skipped != 0 {
		t.Fatalf("missing pending: %+v", report)
	}
}

func TestFlusher_NilClientErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := os.Create(filepath.Join(dir, "pending.jsonl")); err != nil {
		t.Fatalf("touch: %v", err)
	}
	flusher := &Flusher{PendingPath: filepath.Join(dir, "pending.jsonl"), CursorPath: filepath.Join(dir, "cursor")}
	if _, err := flusher.Flush(context.Background()); err == nil {
		t.Fatalf("want nil-client error")
	}
}

func TestFlusher_SkipsCorruptLine(t *testing.T) {
	dir := t.TempDir()
	pending := filepath.Join(dir, "pending.jsonl")
	if err := os.WriteFile(pending, []byte("{not json\n{\"id\":\"ok\",\"text\":\"ok\"}\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	pusher := &fakePusher{}
	flusher := &Flusher{
		PendingPath: pending,
		CursorPath:  filepath.Join(dir, "cursor"),
		Client:      pusher,
		BatchSize:   10,
	}
	report, err := flusher.Flush(context.Background())
	if err != nil {
		t.Fatalf("flush: %v", err)
	}
	if report.Flushed != 1 || report.Skipped != 1 {
		t.Fatalf("want 1 flushed + 1 skipped, got %+v", report)
	}
	if pusher.calls != 1 {
		t.Fatalf("upstream calls=%d", pusher.calls)
	}
}

func TestReader_TailHandlesMissingFile(t *testing.T) {
	r := NewReader(filepath.Join(t.TempDir(), "absent.jsonl"), "")
	caps, err := r.Tail(5)
	if err != nil {
		t.Fatalf("missing file: %v", err)
	}
	if len(caps) != 0 {
		t.Fatalf("want 0, got %d", len(caps))
	}
}

func TestMem0Client_Error4xx(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"detail":"bad"}`))
	}))
	defer upstream.Close()
	cli := NewMem0Client(upstream.URL, "tok")
	cli.HTTP = upstream.Client()
	err := cli.Push(context.Background(), Capsule{ID: "x", Text: "x"})
	if err == nil || !strings.Contains(err.Error(), "HTTP 400") {
		t.Fatalf("want 400, got %v", err)
	}
}

type fakePusher struct {
	calls int
}

func (p *fakePusher) Push(_ context.Context, _ Capsule) error {
	p.calls++
	return nil
}
