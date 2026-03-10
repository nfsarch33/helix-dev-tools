package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nfsarch33/cursor-tools/internal/config"
)

func TestDerivedPathsAndPlatformBranches(t *testing.T) {
	oldHome := os.Getenv("HOME")
	home := t.TempDir()
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatal(err)
	}
	defer os.Setenv("HOME", oldHome)

	p := config.DefaultPaths()
	if got := p.ToolsDir(); got != filepath.Join(p.Memo, "tools") {
		t.Fatalf("ToolsDir() = %q", got)
	}
	if got := p.MetricsFile(); got != filepath.Join(p.HooksDir, "metrics.jsonl") {
		t.Fatalf("MetricsFile() = %q", got)
	}

	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sshDir, "agtc"), []byte("key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := p.SSHKeyPath(); got != filepath.Join(sshDir, "agtc") {
		t.Fatalf("SSHKeyPath() = %q", got)
	}

	_ = os.Remove(filepath.Join(sshDir, "agtc"))
	if err := os.WriteFile(filepath.Join(sshDir, "wsl_ubuntu"), []byte("key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := p.SSHKeyPath(); got != filepath.Join(sshDir, "wsl_ubuntu") {
		t.Fatalf("SSHKeyPath() fallback = %q", got)
	}

	if got := p.PlatformProfile(); got == "" {
		t.Fatal("PlatformProfile() returned empty string")
	}
	// WSL_INTEROP trick only works on Linux; on macOS the darwin branch
	// short-circuits, so we just verify non-empty above.
	if os.Getenv("WSL_INTEROP") != "" || p.PlatformProfile() == "linux" {
		oldWSL := os.Getenv("WSL_INTEROP")
		if err := os.Setenv("WSL_INTEROP", "1"); err != nil {
			t.Fatal(err)
		}
		defer os.Setenv("WSL_INTEROP", oldWSL)
		if got := p.PlatformProfile(); got != "wsl" {
			t.Fatalf("PlatformProfile() = %q, want wsl", got)
		}
	}
}
