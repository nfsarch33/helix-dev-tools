package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

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
		Hooks:        countFiles(p.HooksDir, ".sh"),
		Agents:       countFiles(p.AgentsDir, ".md"),
		Commands:     countFiles(p.CommandsDir, ".md"),
	}
}

type replacement struct {
	file     string
	pattern  string
	template string
}

func runSyncCounts(_ *cobra.Command, _ []string) error {
	p := config.DefaultPaths()
	counts := getDiskCounts(p)

	fmt.Printf("Disk counts:\n")
	fmt.Printf("  Skills: %d (%d cursor + %d agents)\n", counts.TotalSkills, counts.CursorSkills, counts.AgentsSkills)
	fmt.Printf("  Hooks: %d\n", counts.Hooks)
	fmt.Printf("  Sub-agents: %d\n", counts.Agents)
	fmt.Printf("  Commands: %d\n", counts.Commands)
	fmt.Println()

	indexFiles := map[string]string{
		"daily-startup-prompt": filepath.Join(p.GlobalMemoriesDir(), "daily-startup-prompt.md"),
		"skills-index":         filepath.Join(p.GlobalMemoriesDir(), "skills-index.md"),
		"00-index":             filepath.Join(p.SkillsDir, "00-index", "SKILL.md"),
		"one-person-company":   filepath.Join(p.GlobalMemoriesDir(), "one-person-company-progress.md"),
	}

	replacements := []replacement{
		{"daily-startup-prompt", `\(\d+ unique skills\)`, fmt.Sprintf("(%d unique skills)", counts.TotalSkills)},
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
			fmt.Printf("  ERROR: file not found: %s\n", fpath)
			errors++
			continue
		}
		content := string(data)
		re := regexp.MustCompile(r.pattern)
		match := re.FindString(content)
		if match == "" {
			fmt.Printf("  WARN: pattern not found in %s: %s\n", r.file, r.pattern)
			continue
		}
		if match == r.template {
			fmt.Printf("  OK    %s: %s\n", r.file, match)
			continue
		}
		fmt.Printf("  DRIFT %s:\n         was: %s\n         now: %s\n", r.file, match, r.template)
		changes++

		if syncCountsApply {
			updated := strings.Replace(content, match, r.template, 1)
			if err := lockfile.LockedWrite(lockPath, fpath, updated); err != nil {
				fmt.Printf("         ERROR: %v\n", err)
				errors++
			} else {
				fmt.Printf("         APPLIED\n")
			}
		}
	}

	fmt.Println()
	if changes == 0 {
		fmt.Println("No drift detected. All counts match disk.")
	} else if syncCountsApply {
		fmt.Printf("%d file(s) updated.\n", changes)
	} else {
		fmt.Printf("%d drift(s) found. Run with --apply to fix.\n", changes)
	}

	if errors > 0 {
		fmt.Printf("%d error(s) encountered.\n", errors)
		return fmt.Errorf("%d errors", errors)
	}
	return nil
}
