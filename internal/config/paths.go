// runx-public-repo-gate: allow-file secret_cred_ref — code resolves and tests deny-list of literal SSH key paths and identifiers (id_rsa, agtc) for hook fixtures

package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Paths holds all configurable directory paths.
type Paths struct {
	Home            string
	GlobalKB        string
	Memo            string
	HooksDir        string
	SkillsDir       string
	AgentsDir       string
	AgentsSkillsDir string
	CommandsDir     string
	RulesDir        string
	BinDir          string
}

// DefaultPaths returns platform-aware default paths.
func DefaultPaths() Paths {
	home := os.Getenv("HOME")
	if home == "" {
		if runtime.GOOS == "windows" {
			home = os.Getenv("USERPROFILE")
		} else {
			home = "~"
		}
	}

	globalKB := envOr("GLOBAL_KB", filepath.Join(home, "Code", "global-kb"))
	// Memo is retained as a deprecated alias so older call sites do not nil out,
	// but active path resolution is now pinned to the Git-backed global-kb root.
	memo := globalKB

	return Paths{
		Home:            home,
		GlobalKB:        globalKB,
		Memo:            memo,
		HooksDir:        filepath.Join(home, ".cursor", "hooks"),
		SkillsDir:       filepath.Join(home, ".cursor", "skills"),
		AgentsDir:       filepath.Join(home, ".claude", "agents"),
		AgentsSkillsDir: filepath.Join(home, ".agents", "skills"),
		CommandsDir:     filepath.Join(home, ".cursor", "commands"),
		RulesDir:        filepath.Join(home, ".cursor", "rules"),
		BinDir:          filepath.Join(home, "bin"),
	}
}

// CursorConfigDir returns the cursor-config directory within global-kb.
func (p Paths) CursorConfigDir() string {
	return filepath.Join(p.GlobalKB, "cursor-config")
}

// CursorMCPConfig returns the global Cursor MCP config path.
func (p Paths) CursorMCPConfig() string {
	return filepath.Join(p.Home, ".cursor", "mcp.json")
}

// HooksJSONPath returns the live hooks.json symlink path.
func (p Paths) HooksJSONPath() string {
	return filepath.Join(p.Home, ".cursor", "hooks.json")
}

// SkillsCursorDir returns the cursor-managed skills directory.
func (p Paths) SkillsCursorDir() string {
	return filepath.Join(p.Home, ".cursor", "skills-cursor")
}

// GlobalMemoriesDir returns the Git-backed startup-index directory.
func (p Paths) GlobalMemoriesDir() string {
	return filepath.Join(p.GlobalKB, "global-memories")
}

// GlobalLearningsDir returns the global learnings directory.
func (p Paths) GlobalLearningsDir() string {
	return filepath.Join(p.GlobalKB, "learnings")
}

// SOPDir returns the SOP directory.
func (p Paths) SOPDir() string {
	return filepath.Join(p.GlobalKB, "sop")
}

// ToolsDir returns the tools directory within the Git-backed global-kb root.
func (p Paths) ToolsDir() string {
	return filepath.Join(p.GlobalKB, "tools")
}

// LogFile returns the log file path for a named hook.
func (p Paths) LogFile(name string) string {
	return filepath.Join(p.HooksDir, name+".log")
}

// LockDir returns the lock directory path for a named lock.
func (p Paths) LockDir(name string) string {
	return filepath.Join(p.HooksDir, "."+name+".lock")
}

// LockFile returns the lock file path for a named lock.
func (p Paths) LockFile(name string) string {
	return filepath.Join(p.HooksDir, "."+name+".lock")
}

// LogDir returns the runx-style log directory ($HOME/logs/runx).
func (p Paths) LogDir() string {
	return filepath.Join(p.Home, "logs", "runx")
}

// MetricsFile returns the path to the JSONL metrics file.
func (p Paths) MetricsFile() string {
	return filepath.Join(p.HooksDir, "metrics.jsonl")
}

// PlatformProfile returns the expected host profile for Cursor tooling.
func (p Paths) PlatformProfile() string {
	switch {
	case runtime.GOOS == "darwin":
		return "macos"
	case isWSL():
		return "wsl"
	default:
		return runtime.GOOS
	}
}

// PlatformBinarySuffix returns the dist binary suffix for the current host.
func (p Paths) PlatformBinarySuffix() string {
	goos := runtime.GOOS
	if isWSL() {
		goos = "linux"
	}
	return goos + "-" + runtime.GOARCH
}

// SSHKeyPath returns the path to the SSH private key used for GitHub.
// Checks for ~/.ssh/agtc first (macOS), falls back to ~/.ssh/wsl_ubuntu (WSL).
func (p Paths) SSHKeyPath() string {
	for _, name := range []string{"agtc", "wsl_ubuntu"} {
		path := filepath.Join(p.Home, ".ssh", name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return filepath.Join(p.Home, ".ssh", "agtc")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func isWSL() bool {
	if os.Getenv("WSL_INTEROP") != "" || os.Getenv("WSL_DISTRO_NAME") != "" {
		return true
	}
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), "microsoft")
}
