package zdproxy

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// minLocalTokenBytes is the minimum decoded length we accept when reusing
// an existing token file. NewLocalToken mints 32 bytes of entropy; we
// allow 32 as the floor to avoid silently accepting truncated files. A
// malformed or short token is rejected and a fresh one is minted.
const minLocalTokenBytes = 32

// WriteLocalTokenFile writes the per-process local auth token to disk with
// owner-only permissions (0600). The parent directory is created with mode
// 0700 if missing. The file content is `<token>\n` so shells can `cat` it
// without surprises.
func WriteLocalTokenFile(path, token string) error {
	if path == "" {
		return fmt.Errorf("local token path must be non-empty")
	}
	if token == "" {
		return fmt.Errorf("local token must be non-empty")
	}
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("create token dir %q: %w", dir, err)
		}
	}
	tmp, err := os.CreateTemp(dir, ".local-token-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp token file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.WriteString(token + "\n"); err != nil {
		tmp.Close()
		return fmt.Errorf("write token: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod token: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close token: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename token file: %w", err)
	}
	return nil
}

// DefaultLocalTokenPath returns the default path for the per-process local
// auth token. It honours `XDG_CONFIG_HOME` and falls back to
// `$HOME/.config/zd-claude-proxy/local-token`.
func DefaultLocalTokenPath() (string, error) {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "zd-claude-proxy", "local-token"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".config", "zd-claude-proxy", "local-token"), nil
}

// ResolveLocalToken returns a usable local-auth token for the proxy,
// either by reusing the value at `path` or by minting a fresh one.
//
// When reuseExisting is true, an existing file is honoured if its content
// is a non-empty base64url string of at least minLocalTokenBytes bytes
// after decoding. Anything shorter, malformed, or unreadable triggers a
// fresh mint + write. The `written` flag tells the caller whether the
// disk was touched (useful for log gating + filesystem audit).
//
// When reuseExisting is false, a fresh token is always minted and
// written. This is the contract the operator opts in to when they want
// strict per-process rotation.
//
// All on-disk writes go through WriteLocalTokenFile, so 0600 mode and
// 0700 parent-dir mode are inherited.
func ResolveLocalToken(path string, reuseExisting bool) (token string, written bool, err error) {
	if reuseExisting {
		if existing, ok := readReusableToken(path); ok {
			return existing, false, nil
		}
	}
	tok, err := NewLocalToken()
	if err != nil {
		return "", false, fmt.Errorf("mint token: %w", err)
	}
	if err := WriteLocalTokenFile(path, tok); err != nil {
		return "", false, err
	}
	return tok, true, nil
}

// readReusableToken returns (token, true) if `path` holds a syntactically
// valid token that is safe to reuse, otherwise ("", false). Errors
// (file missing, permission denied, decoder mismatch) all degrade to
// false; the caller falls back to minting.
func readReusableToken(path string) (string, bool) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	tok := strings.TrimSpace(string(body))
	if tok == "" {
		return "", false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(tok)
	if err != nil {
		return "", false
	}
	if len(decoded) < minLocalTokenBytes {
		return "", false
	}
	return tok, true
}
