package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/cursor-tools/internal/clilog"
	"github.com/nfsarch33/cursor-tools/internal/config"
	"github.com/nfsarch33/cursor-tools/internal/lockfile"
)

var syncCountsApply bool

var syncCountsCmd = &cobra.Command{
	Use:   "sync-counts",
	Short: "Count skills/hooks on disk and update index files to match",
	RunE:  runSyncCounts,
}

func init() {
	syncCountsCmd.Flags().BoolVar(&syncCountsApply, "apply", false, "Apply changes in-place")
}

type diskCounts struct {
	CursorSkills int
	AgentsSkills int
	TotalSkills  int
	Hooks        int
	Agents       int
	Commands     int
}

func countHookRoutes(hooksJSONPath string) int {
	data, err := os.ReadFile(hooksJSONPath)
	if err != nil {
		return 0
	}
	content := string(data)
	return strings.Count(content, "cursor-tools hook ")
}

func countSkillDirs(base string, exclude map[string]bool) int {
	entries, err := os.ReadDir(base)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if exclude != nil && exclude[e.Name()] {
			continue
		}
		skillFile := filepath.Join(base, e.Name(), "SKILL.md")
		if _, err := os.Stat(skillFile); err == nil {
			count++
		}
	}
	return count
}

func countFiles(dir, ext string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ext) {
			count++
		}
	}
	return count
}

func getDiskCounts(p config.Paths) diskCounts {
	cursor := countSkillDirs(p.SkillsDir, map[string]bool{"00-index": true})
	agents := countSkillDirs(p.AgentsSkillsDir, nil)
	return diskCounts{
		CursorSkills: cursor,
		AgentsSkills: agents,
		TotalSkills:  cursor + agents,
		Hooks:        countHookRoutes(filepath.Join(p.Home, ".cursor", "hooks.json")),
		Agents:       countFiles(p.AgentsDir, ".md"),
		Commands:     countFiles(p.CommandsDir, ".md"),
	}
}

type replacement struct {
	file     string
	pattern  string
	template string
}

// SyncCountsApply counts skills/hooks on disk and updates index files.
// When apply is true, files are rewritten in place. When quiet is true,
// only drift and errors are printed (used by health-check).
// Returns (changes, errors).
func SyncCountsApply(apply, quiet bool) (int, int) {
	p := config.DefaultPaths()
	counts := getDiskCounts(p)

	if !quiet {
		fmt.Println("Disk counts:")
		clilog.Info("Skills: %d (%d cursor + %d agents)", counts.TotalSkills, counts.CursorSkills, counts.AgentsSkills)
		clilog.Info("Hooks: %d", counts.Hooks)
		clilog.Info("Sub-agents: %d", counts.Agents)
		clilog.Info("Commands: %d", counts.Commands)
		fmt.Println()
	}

	indexFiles := map[string]string{
		"daily-startup-prompt": filepath.Join(p.GlobalMemoriesDir(), "daily-startup-prompt.md"),
		"skills-index":         filepath.Join(p.GlobalMemoriesDir(), "skills-index.md"),
		"00-index":             filepath.Join(p.SkillsDir, "00-index", "SKILL.md"),
		"one-person-company":   filepath.Join(p.GlobalMemoriesDir(), "one-person-company-progress.md"),
	}

	replacements := []replacement{
		{"daily-startup-prompt", `\(\d+ unique skills, \d+ L0 rules\)`, fmt.Sprintf("(%d unique skills, 10 L0 rules)", counts.TotalSkills)},
		{"daily-startup-prompt", `Slash commands: \d+ in`, fmt.Sprintf("Slash commands: %d in", counts.Commands)},
		{"skills-index", `Total: \d+ unique skills across ~/\.cursor/skills/ \(\d+\) and ~/\.agents/skills/ \(\d+\)`,
			fmt.Sprintf("Total: %d unique skills across ~/.cursor/skills/ (%d) and ~/.agents/skills/ (%d)", counts.TotalSkills, counts.CursorSkills, counts.AgentsSkills)},
		{"00-index", `## Skills \(\d+ unique across`, fmt.Sprintf("## Skills (%d unique across", counts.TotalSkills)},
		{"one-person-company", `### Agent Skills \(\d+ unique across two dirs`, fmt.Sprintf("### Agent Skills (%d unique across two dirs", counts.TotalSkills)},
	}

	changes := 0
	errors := 0
	lockPath := filepath.Join(p.HooksDir, ".sync-counts.lock")

	for _, r := range replacements {
		fpath, ok := indexFiles[r.file]
		if !ok {
			continue
		}
		data, err := os.ReadFile(fpath)
		if err != nil {
			if !quiet {
				clilog.Error("file not found: %s", fpath)
			}
			errors++
			continue
		}
		content := string(data)
		re := regexp.MustCompile(r.pattern)
		match := re.FindString(content)
		if match == "" {
			if !quiet {
				clilog.Warn("pattern not found in %s: %s", r.file, r.pattern)
			}
			continue
		}
		if match == r.template {
			if !quiet {
				clilog.Success("%s: %s", r.file, match)
			}
			continue
		}
		if !quiet {
			clilog.Warn("DRIFT %s:\n         was: %s\n         now: %s", r.file, match, r.template)
		}
		changes++

		if apply {
			updated := strings.Replace(content, match, r.template, 1)
			if err := lockfile.LockedWrite(lockPath, fpath, updated); err != nil {
				if !quiet {
					clilog.Error("write failed: %v", err)
				}
				errors++
			} else if !quiet {
				clilog.Success("APPLIED")
			}
		}
	}

	if !quiet {
		fmt.Println()
		if changes == 0 {
			clilog.Success("No drift detected. All counts match disk.")
		} else if apply {
			clilog.Info("%d file(s) updated.", changes)
		} else {
			clilog.Warn("%d drift(s) found. Run with --apply to fix.", changes)
		}
	}
	return changes, errors
}

func runSyncCounts(_ *cobra.Command, _ []string) error {
	_, errors := SyncCountsApply(syncCountsApply, false)
	if errors > 0 {
		return fmt.Errorf("%d errors", errors)
	}
	return nil
}
