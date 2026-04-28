package zdproxy

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriteLocalTokenFile_PermissionsAreUserOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix permission test not relevant on windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "local-token")
	if err := WriteLocalTokenFile(path, "secret-token"); err != nil {
		t.Fatalf("WriteLocalTokenFile: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if got, want := info.Mode().Perm(), fs.FileMode(0600); got != want {
		t.Fatalf("expected mode %o, got %o", want, got)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(body) != "secret-token\n" {
		t.Fatalf("unexpected file body %q", body)
	}
}

func TestWriteLocalTokenFile_CreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "local-token")
	if err := WriteLocalTokenFile(path, "x"); err != nil {
		t.Fatalf("WriteLocalTokenFile: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("token file not created: %v", err)
	}
}
