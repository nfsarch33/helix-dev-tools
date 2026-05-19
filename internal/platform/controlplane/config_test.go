package controlplane

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigStore_SetAndGet(t *testing.T) {
	c := NewConfigStore("")
	c.Set("key1", "value1")

	v, ok := c.Get("key1")
	if !ok || v != "value1" {
		t.Errorf("expected value1, got %v", v)
	}
}

func TestConfigStore_GetString(t *testing.T) {
	c := NewConfigStore("")
	c.Set("name", "helixon")

	if c.GetString("name", "") != "helixon" {
		t.Error("expected helixon")
	}
	if c.GetString("missing", "default") != "default" {
		t.Error("expected default for missing key")
	}
}

func TestConfigStore_LoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte(`{"port": 8080, "host": "localhost"}`), 0644)

	c := NewConfigStore(path)
	if err := c.LoadFromFile(); err != nil {
		t.Fatalf("load error: %v", err)
	}

	v, ok := c.Get("host")
	if !ok || v != "localhost" {
		t.Errorf("expected localhost, got %v", v)
	}
}

func TestConfigStore_LoadFromFile_Invalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte(`not json`), 0644)

	c := NewConfigStore(path)
	if err := c.LoadFromFile(); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestConfigStore_Validate(t *testing.T) {
	c := NewConfigStore("")
	c.Set("a", 1)
	c.Set("b", 2)

	missing := c.Validate([]string{"a", "b", "c"})
	if len(missing) != 1 || missing[0] != "c" {
		t.Errorf("expected [c] missing, got %v", missing)
	}
}

func TestConfigStore_All(t *testing.T) {
	c := NewConfigStore("")
	c.Set("x", 1)
	c.Set("y", 2)

	all := c.All()
	if len(all) != 2 {
		t.Errorf("expected 2 entries, got %d", len(all))
	}
}
