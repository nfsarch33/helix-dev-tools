package installcheck

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckBinaries_ExistingFile(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "mybin")
	if err := os.WriteFile(bin, []byte("#!/bin/sh"), 0755); err != nil {
		t.Fatal(err)
	}

	results := CheckBinaries([]string{bin})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusPass {
		t.Fatalf("expected pass, got %s: %s", results[0].Status, results[0].Error)
	}
}

func TestCheckBinaries_MissingFile(t *testing.T) {
	results := CheckBinaries([]string{"/nonexistent/path/bin"})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusMissing {
		t.Fatalf("expected missing, got %s", results[0].Status)
	}
}

func TestCheckConfigs_Mixed(t *testing.T) {
	tmp := t.TempDir()
	cfg := filepath.Join(tmp, "config.yaml")
	if err := os.WriteFile(cfg, []byte("key: value"), 0644); err != nil {
		t.Fatal(err)
	}

	results := CheckConfigs([]string{cfg, "/nonexistent/config.yaml"})
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Status != StatusPass {
		t.Fatalf("expected pass for existing config, got %s", results[0].Status)
	}
	if results[1].Status != StatusMissing {
		t.Fatalf("expected missing for nonexistent config, got %s", results[1].Status)
	}
}
