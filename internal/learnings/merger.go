package learnings

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/nfsarch33/cursor-tools/internal/lockfile"
)

// WorkspaceResult holds counts of promoted items from a workspace.
type WorkspaceResult struct {
	Entries  int
	Patterns int
	Episodes int
}

// PromoteWorkspace promotes project .learnings/ content to the global learnings directory.
func PromoteWorkspace(workspacePath, globalLearningsDir string, dryRun bool) WorkspaceResult {
	projectDir := filepath.Join(workspacePath, ".learnings")
	if !isDir(projectDir) {
		return WorkspaceResult{}
	}

	result := WorkspaceResult{}

	for _, filename := range []string{"ERRORS.md", "LEARNINGS.md", "FEATURE_REQUESTS.md"} {
		src := filepath.Join(projectDir, filename)
		tgt := filepath.Join(globalLearningsDir, filename)
		if fileExists(src) && fileExists(tgt) {
			result.Entries += mergeEntries(src, tgt, dryRun)
		}
	}

	srcPatterns := filepath.Join(projectDir, "PATTERNS.md")
	tgtPatterns := filepath.Join(globalLearningsDir, "PATTERNS.md")
	if fileExists(srcPatterns) && fileExists(tgtPatterns) {
		result.Patterns = mergePatterns(srcPatterns, tgtPatterns, dryRun)
	}

	srcEpisodes := filepath.Join(projectDir, "episodes")
	if isDir(srcEpisodes) {
		tgtEpisodes := filepath.Join(globalLearningsDir, "episodes")
		if !dryRun {
			_ = os.MkdirAll(tgtEpisodes, 0o755)
		}
		entries, err := os.ReadDir(srcEpisodes)
		if err == nil {
			for _, e := range entries {
				if !strings.HasSuffix(e.Name(), ".md") {
					continue
				}
				tgtPath := filepath.Join(tgtEpisodes, e.Name())
				if _, err := os.Stat(tgtPath); err == nil {
					continue
				}
				if !dryRun {
					data, err := os.ReadFile(filepath.Join(srcEpisodes, e.Name()))
					if err == nil {
						_ = os.WriteFile(tgtPath, data, 0o644) // #nosec G703 -- path from trusted config, not user input
					}
				}
				result.Episodes++
			}
		}
	}

	return result
}

func mergeEntries(srcFile, tgtFile string, dryRun bool) int {
	srcEntries, _ := ParseEntries(srcFile)
	tgtEntries, _ := ParseEntries(tgtFile)

	tgtFP := make(map[string]bool)
	for _, e := range tgtEntries {
		tgtFP[e.Fingerprint] = true
	}

	var newEntries []Entry
	for _, e := range srcEntries {
		if !tgtFP[e.Fingerprint] {
			newEntries = append(newEntries, e)
		}
	}

	if len(newEntries) == 0 || dryRun {
		return len(newEntries)
	}

	existing, _ := os.ReadFile(tgtFile)
	additions := make([]string, len(newEntries))
	for i, e := range newEntries {
		additions[i] = e.Content
	}
	content := strings.TrimRight(string(existing), "\n") + "\n\n" + strings.Join(additions, "\n\n") + "\n"
	_ = lockfile.LockedWrite(lockFilePath(), tgtFile, content)
	return len(newEntries)
}

func mergePatterns(srcFile, tgtFile string, dryRun bool) int {
	srcPats, _ := ParsePatterns(srcFile)
	tgtPats, _ := ParsePatterns(tgtFile)

	newCount := 0
	for id, src := range srcPats {
		if existing, ok := tgtPats[id]; ok {
			if src.Applications > existing.Applications {
				tgtPats[id] = src
				newCount++
			}
		} else {
			tgtPats[id] = src
			newCount++
		}
	}

	if newCount == 0 || dryRun {
		return newCount
	}

	data, _ := os.ReadFile(tgtFile)
	lines := strings.Split(string(data), "\n")

	var header []string
	for _, line := range lines {
		header = append(header, line)
		if strings.HasPrefix(strings.TrimSpace(line), "|----") {
			break
		}
	}

	sorted := make([]Pattern, 0, len(tgtPats))
	for _, p := range tgtPats {
		sorted = append(sorted, p)
	}
	sort.Slice(sorted, func(i, j int) bool {
		ni, _ := strconv.Atoi(strings.TrimPrefix(sorted[i].ID, "pat-"))
		nj, _ := strconv.Atoi(strings.TrimPrefix(sorted[j].ID, "pat-"))
		return ni < nj
	})

	var result strings.Builder
	for _, h := range header {
		result.WriteString(h + "\n")
	}
	for _, p := range sorted {
		result.WriteString(p.RawLine + "\n")
	}

	_ = lockfile.LockedWrite(lockFilePath(), tgtFile, result.String())
	return newCount
}

func lockFilePath() string {
	home := os.Getenv("HOME")
	return filepath.Join(home, ".cursor", "hooks", ".promote-learnings.lock")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
