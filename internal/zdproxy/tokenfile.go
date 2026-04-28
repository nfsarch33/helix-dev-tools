package zdproxy

import (
	"fmt"
	"os"
	"path/filepath"
)

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
