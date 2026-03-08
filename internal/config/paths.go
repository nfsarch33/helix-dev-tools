package config

import (
	"os"
	"path/filepath"
	"runtime"
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
	memo := envOr("MEMO", filepath.Join(home, "memo"))

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

// GlobalMemoriesDir returns the Pepper L1 data directory.
func (p Paths) GlobalMemoriesDir() string {
	return filepath.Join(p.Memo, "global-memories")
}

// GlobalLearningsDir returns the global learnings directory.
func (p Paths) GlobalLearningsDir() string {
	return filepath.Join(p.Memo, "learnings")
}

// SOPDir returns the SOP directory.
func (p Paths) SOPDir() string {
	return filepath.Join(p.GlobalKB, "sop")
}

// ToolsDir returns the tools directory within memo (~/memo/tools).
func (p Paths) ToolsDir() string {
	return filepath.Join(p.Memo, "tools")
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
