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

// ResolveLocalToken — v257 W1 D1 ADR-020 contract. The proxy must be able
// to reuse a stable on-disk token across restarts so that long-lived
// clients (~/.claude/settings.json, ~/.codex/config.toml) don't have to
// be regenerated on every proxy bounce.
//
// The contract:
//
//   - reuseExisting=true  + readable file with ≥ 32 base64url chars  -> reuse, written=false
//   - reuseExisting=true  + missing file                              -> mint new, written=true
//   - reuseExisting=true  + readable file with malformed token        -> mint new, written=true
//   - reuseExisting=false                                             -> always mint new, written=true
//
// The malformed-token branch protects us from an operator hand-editing the
// token file or a half-written file from a previous crash.

func TestResolveLocalToken_ReuseExisting(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix permission test not relevant on windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "local-token")
	first, _, err := ResolveLocalToken(path, true)
	if err != nil {
		t.Fatalf("first ResolveLocalToken: %v", err)
	}
	if len(first) < 32 {
		t.Fatalf("expected token len >= 32, got %d", len(first))
	}
	second, written, err := ResolveLocalToken(path, true)
	if err != nil {
		t.Fatalf("second ResolveLocalToken: %v", err)
	}
	if second != first {
		t.Fatalf("expected token reuse, got %q vs first %q", second, first)
	}
	if written {
		t.Fatalf("expected written=false on reuse")
	}
}

func TestResolveLocalToken_MintNew_WhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "local-token")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("precondition: file should be missing")
	}
	tok, written, err := ResolveLocalToken(path, true)
	if err != nil {
		t.Fatalf("ResolveLocalToken: %v", err)
	}
	if !written {
		t.Fatalf("expected written=true when file missing")
	}
	if len(tok) < 32 {
		t.Fatalf("expected token len >= 32, got %d", len(tok))
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("token file should exist: %v", err)
	}
	if got := string(body); got != tok+"\n" {
		t.Fatalf("file body %q does not match returned token %q", got, tok)
	}
}

func TestResolveLocalToken_AlwaysMintWhenReuseFalse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "local-token")
	first, _, err := ResolveLocalToken(path, true)
	if err != nil {
		t.Fatalf("seed ResolveLocalToken: %v", err)
	}
	second, written, err := ResolveLocalToken(path, false)
	if err != nil {
		t.Fatalf("rotate ResolveLocalToken: %v", err)
	}
	if !written {
		t.Fatalf("expected written=true on forced rotation")
	}
	if second == first {
		t.Fatalf("expected fresh token, got reuse %q", second)
	}
}

func TestResolveLocalToken_RejectMalformed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "local-token")
	if err := os.WriteFile(path, []byte("# not a token\n"), 0o600); err != nil {
		t.Fatalf("seed malformed: %v", err)
	}
	tok, written, err := ResolveLocalToken(path, true)
	if err != nil {
		t.Fatalf("ResolveLocalToken: %v", err)
	}
	if !written {
		t.Fatalf("expected written=true when existing token malformed")
	}
	if len(tok) < 32 {
		t.Fatalf("expected fresh token, got len %d", len(tok))
	}
	body, _ := os.ReadFile(path)
	if string(body) != tok+"\n" {
		t.Fatalf("file body %q does not match returned token %q", string(body), tok)
	}
}

func TestResolveLocalToken_RejectShortToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "local-token")
	if err := os.WriteFile(path, []byte("abcdef\n"), 0o600); err != nil {
		t.Fatalf("seed short: %v", err)
	}
	tok, written, err := ResolveLocalToken(path, true)
	if err != nil {
		t.Fatalf("ResolveLocalToken: %v", err)
	}
	if !written {
		t.Fatalf("expected written=true when existing token too short")
	}
	if len(tok) < 32 {
		t.Fatalf("expected fresh token, got len %d", len(tok))
	}
}
