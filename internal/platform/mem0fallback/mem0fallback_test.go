package mem0fallback

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewCreatesDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "outbox.json")
	o, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if o == nil {
		t.Fatal("expected non-nil outbox")
	}
}

func TestEnqueueAndPending(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outbox.json")
	o, _ := New(path)
	o.Enqueue("hello world", "user1", "app1", map[string]string{"key": "val"})
	o.Enqueue("second entry", "user1", "app1", nil)

	pending := o.Pending()
	if len(pending) != 2 {
		t.Fatalf("expected 2, got %d", len(pending))
	}
	if pending[0].Text != "hello world" {
		t.Errorf("text mismatch: %s", pending[0].Text)
	}
	if pending[0].Metadata["key"] != "val" {
		t.Error("metadata missing")
	}
}

func TestLen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outbox.json")
	o, _ := New(path)
	if o.Len() != 0 {
		t.Error("empty outbox should have len 0")
	}
	o.Enqueue("x", "u", "a", nil)
	if o.Len() != 1 {
		t.Errorf("expected 1, got %d", o.Len())
	}
}

func TestRemove(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outbox.json")
	o, _ := New(path)
	o.Enqueue("first", "u", "a", nil)
	o.Enqueue("second", "u", "a", nil)
	o.Remove(0)
	if o.Len() != 1 {
		t.Fatalf("expected 1 after remove, got %d", o.Len())
	}
	if o.Pending()[0].Text != "second" {
		t.Error("wrong entry remained")
	}
}

func TestMarkFailed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outbox.json")
	o, _ := New(path)
	o.Enqueue("will fail", "u", "a", nil)
	o.MarkFailed(0, "timeout")
	o.MarkFailed(0, "timeout again")
	pending := o.Pending()
	if pending[0].Attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", pending[0].Attempts)
	}
	if pending[0].LastError != "timeout again" {
		t.Errorf("last error: %s", pending[0].LastError)
	}
}

func TestClear(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outbox.json")
	o, _ := New(path)
	o.Enqueue("x", "u", "a", nil)
	o.Enqueue("y", "u", "a", nil)
	o.Clear()
	if o.Len() != 0 {
		t.Errorf("expected 0 after clear, got %d", o.Len())
	}
}

func TestPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outbox.json")
	o1, _ := New(path)
	o1.Enqueue("persisted", "u", "a", nil)

	o2, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if o2.Len() != 1 {
		t.Fatalf("expected 1 from disk, got %d", o2.Len())
	}
	if o2.Pending()[0].Text != "persisted" {
		t.Error("wrong text from disk")
	}
}

func TestRemoveOutOfBounds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outbox.json")
	o, _ := New(path)
	o.Enqueue("x", "u", "a", nil)
	o.Remove(-1)
	o.Remove(99)
	if o.Len() != 1 {
		t.Error("out-of-bounds remove should be no-op")
	}
}

func TestDefaultPath(t *testing.T) {
	t.Setenv("HOME", "/tmp/test-home")
	p := DefaultPath()
	if p != "/tmp/test-home/.config/helix-dev-tools/mem0-outbox.json" {
		t.Errorf("unexpected: %s", p)
	}
}

func TestNewWithExistingCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outbox.json")
	os.WriteFile(path, []byte("not json"), 0600)
	_, err := New(path)
	if err == nil {
		t.Fatal("expected error for corrupt file")
	}
}
