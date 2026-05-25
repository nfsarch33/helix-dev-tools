package ndjsonutil

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type testEvent struct {
	ID  int    `json:"id"`
	Msg string `json:"msg"`
}

func writeTestFile(t *testing.T, path string, events []testEvent) {
	t.Helper()
	w, err := Open(path, WithMaxBytes(0))
	if err != nil {
		t.Fatalf("writeTestFile Open: %v", err)
	}
	for _, e := range events {
		if err := w.Append(e); err != nil {
			t.Fatalf("writeTestFile Append: %v", err)
		}
	}
	w.Close()
}

func TestOpenReader_ReadAll(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "read.ndjson")
	events := []testEvent{{ID: 1, Msg: "a"}, {ID: 2, Msg: "b"}, {ID: 3, Msg: "c"}}
	writeTestFile(t, path, events)

	got, err := ReadAll[testEvent](path)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("ReadAll returned %d items, want 3", len(got))
	}
	for i, e := range got {
		if e.ID != events[i].ID || e.Msg != events[i].Msg {
			t.Errorf("item %d: got %+v, want %+v", i, e, events[i])
		}
	}
}

func TestOpenReader_Next(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "next.ndjson")
	writeTestFile(t, path, []testEvent{{ID: 42, Msg: "hello"}})

	r, err := OpenReader(path)
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	defer r.Close()

	var ev testEvent
	if err := r.Next(&ev); err != nil {
		t.Fatalf("Next: %v", err)
	}
	if ev.ID != 42 {
		t.Fatalf("got ID=%d, want 42", ev.ID)
	}

	if err := r.Next(&ev); err != io.EOF {
		t.Fatalf("expected EOF, got %v", err)
	}
}

func TestOpenReader_EmptyFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.ndjson")
	os.WriteFile(path, []byte{}, 0o644)

	r, err := OpenReader(path)
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	defer r.Close()

	var ev testEvent
	if err := r.Next(&ev); err != io.EOF {
		t.Fatalf("expected EOF on empty file, got %v", err)
	}
}

func TestOpenReader_InvalidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.ndjson")
	os.WriteFile(path, []byte("not json\n"), 0o644)

	r, err := OpenReader(path)
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	defer r.Close()

	var ev testEvent
	if err := r.Next(&ev); err == nil {
		t.Fatal("expected error on invalid JSON")
	}
}

func TestReadAll_FileNotFound(t *testing.T) {
	t.Parallel()
	_, err := ReadAll[testEvent]("/nonexistent/path.ndjson")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestTailer_Follow(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "tail.ndjson")

	os.WriteFile(path, []byte{}, 0o644)

	tailer, err := NewTailer(path, WithPollInterval(50*time.Millisecond))
	if err != nil {
		t.Fatalf("NewTailer: %v", err)
	}
	defer tailer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var mu sync.Mutex
	var received []testEvent

	go func() {
		_ = tailer.Follow(ctx, func(line []byte) error {
			var ev testEvent
			if err := json.Unmarshal(line, &ev); err != nil {
				return err
			}
			mu.Lock()
			received = append(received, ev)
			mu.Unlock()
			return nil
		})
	}()

	time.Sleep(100 * time.Millisecond)

	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	for i := 0; i < 5; i++ {
		data, _ := json.Marshal(testEvent{ID: i, Msg: "tailed"})
		f.Write(append(data, '\n'))
	}
	f.Close()

	time.Sleep(500 * time.Millisecond)
	cancel()

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 5 {
		t.Fatalf("tailer received %d events, want 5", len(received))
	}
	for i, ev := range received {
		if ev.ID != i {
			t.Errorf("event %d: ID=%d, want %d", i, ev.ID, i)
		}
	}
}

func TestTailer_CancelledContext(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "cancel.ndjson")
	os.WriteFile(path, []byte{}, 0o644)

	tailer, err := NewTailer(path, WithPollInterval(50*time.Millisecond))
	if err != nil {
		t.Fatalf("NewTailer: %v", err)
	}
	defer tailer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = tailer.Follow(ctx, func(line []byte) error { return nil })
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestTailer_Close(t *testing.T) {
	t.Parallel()
	var nilTailer *Tailer
	if err := nilTailer.Close(); err != nil {
		t.Fatalf("nil Close: %v", err)
	}
}
