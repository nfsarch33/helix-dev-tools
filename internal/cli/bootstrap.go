package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/cursor-tools/internal/config"
)

var bootstrapDryRun bool

var bootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Create all symlinks on a fresh machine",
	RunE:  runBootstrap,
}

func init() {
	bootstrapCmd.Flags().BoolVar(&bootstrapDryRun, "dry-run", false, "Show what would happen without making changes")
}

func runBootstrap(_ *cobra.Command, _ []string) error {
	p := config.DefaultPaths()
	globalKB := p.GlobalKB
	cursorConfig := p.CursorConfigDir()

	if bootstrapDryRun {
		fmt.Println("[bootstrap] DRY-RUN MODE: no changes will be made")
	}
	fmt.Printf("[bootstrap] unified-memory: %s\n", globalKB)

	// ~/memo symlink
	memoLink := filepath.Join(p.Home, "memo")
	if info, err := os.Lstat(memoLink); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			target, _ := os.Readlink(memoLink)
			fmt.Printf("[bootstrap] ~/memo symlink already exists -> %s\n", target)
		} else if info.IsDir() {
			fmt.Println("[bootstrap] WARN: ~/memo is a real directory (legacy). Back up and replace.")
		}
	} else {
		fmt.Printf("[bootstrap] Creating ~/memo symlink -> %s\n", globalKB)
		if !bootstrapDryRun {
			_ = os.Symlink(globalKB, memoLink)
		}
	}

	// Create parent directories
	dirs := []string{
		filepath.Join(p.Home, ".cursor", "hooks"),
		filepath.Join(p.Home, ".claude"),
		filepath.Join(p.Home, ".agents"),
		filepath.Join(p.Home, "bin"),
	}
	for _, d := range dirs {
		if !bootstrapDryRun {
			_ = os.MkdirAll(d, 0o755)
		}
	}

	// Directory-level symlinks
	fmt.Println("[bootstrap] Creating directory-level symlinks...")
	dirLinks := [][2]string{
		{p.SkillsDir, filepath.Join(cursorConfig, "skills")},
		{p.RulesDir, filepath.Join(cursorConfig, "rules")},
		{p.CommandsDir, filepath.Join(cursorConfig, "commands")},
		{p.AgentsDir, filepath.Join(cursorConfig, "agents")},
		{p.AgentsSkillsDir, filepath.Join(cursorConfig, "agents-skills")},
	}
	for _, pair := range dirLinks {
		safeSymlink(pair[0], pair[1], bootstrapDryRun)
	}

	// Hook file-level symlinks
	fmt.Println("[bootstrap] Note: hooks are now handled by cursor-tools binary")

	// hooks.json symlink
	hooksJSON := filepath.Join(p.Home, ".cursor", "hooks.json")
	hooksJSONTarget := filepath.Join(cursorConfig, "hooks.json")
	safeSymlink(hooksJSON, hooksJSONTarget, bootstrapDryRun)

	// Install cursor-tools binary to ~/bin
	selfBin, err := os.Executable()
	if err == nil {
		destBin := filepath.Join(p.BinDir, "cursor-tools")
		fmt.Printf("[bootstrap] Installing binary: %s -> %s\n", selfBin, destBin)
		if !bootstrapDryRun {
			data, err := os.ReadFile(selfBin)
			if err == nil {
				_ = os.WriteFile(destBin, data, 0o755)
			}
		}
	}

	// Git hooks path
	gitHooksDir := filepath.Join(cursorConfig, "git-hooks")
	if isDir(gitHooksDir) {
		fmt.Printf("[bootstrap] Setting core.hooksPath -> %s\n", gitHooksDir)
		if !bootstrapDryRun {
			gitCmd("", "config", "--global", "core.hooksPath", gitHooksDir)
		}
	}

	// Allow main push for personal repo
	if isDir(filepath.Join(globalKB, ".git")) {
		fmt.Printf("[bootstrap] hooks.allowMainPush = true for %s\n", globalKB)
		if !bootstrapDryRun {
			gitCmd(globalKB, "config", "hooks.allowMainPush", "true")
		}
	}

	// Chmod
	if !bootstrapDryRun {
		chmodDir(filepath.Join(cursorConfig, "git-hooks"), 0o755)
	}

	fmt.Println("[bootstrap] Restore complete.")
	fmt.Println("[bootstrap] Verify: cursor-tools health-check")
	return nil
}

func safeSymlink(linkPath, target string, dryRun bool) {
	if dryRun {
		fmt.Printf("  [dry-run] ln -s %s %s\n", target, linkPath)
		return
	}
	info, err := os.Lstat(linkPath)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			_ = os.Remove(linkPath)
		} else if info.IsDir() {
			backup := linkPath + ".bak"
			_ = os.Rename(linkPath, backup)
			fmt.Printf("  [backup] %s -> %s\n", linkPath, backup)
		}
	}
	_ = os.Symlink(target, linkPath)
	fmt.Printf("  %s -> %s\n", linkPath, target)
}

func chmodDir(dir string, mode os.FileMode) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		_ = os.Chmod(filepath.Join(dir, e.Name()), mode)
	}
}
