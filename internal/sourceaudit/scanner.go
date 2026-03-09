package sourceaudit

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var sourceRe = regexp.MustCompile(`<!--\s*Source:\s*(https?://\S+)\s*-->`)

// SkillResult holds the source audit result for a single skill.
type SkillResult struct {
	Name       string   `json:"name"`
	Path       string   `json:"path"`
	HasSources bool     `json:"has_sources"`
	Sources    []string `json:"sources"`
}

// ScanSkill scans a single skill directory for Source URL markers.
func ScanSkill(skillDir string) (*SkillResult, error) {
	name := filepath.Base(skillDir)
	skillMD := filepath.Join(skillDir, "SKILL.md")

	result := &SkillResult{
		Name:    name,
		Path:    skillDir,
		Sources: []string{},
	}

	f, err := os.Open(skillMD)
	if err != nil {
		return result, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		matches := sourceRe.FindStringSubmatch(line)
		if len(matches) > 1 {
			url := strings.TrimSpace(matches[1])
			result.Sources = append(result.Sources, url)
		}
	}

	result.HasSources = len(result.Sources) > 0
	return result, scanner.Err()
}

// ScanAll scans all skill directories under the given root.
func ScanAll(root string) ([]SkillResult, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	var results []SkillResult
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillMD := filepath.Join(root, entry.Name(), "SKILL.md")
		if _, statErr := os.Stat(skillMD); os.IsNotExist(statErr) {
			continue
		}
		result, scanErr := ScanSkill(filepath.Join(root, entry.Name()))
		if scanErr != nil {
			continue
		}
		results = append(results, *result)
	}

	return results, nil
}
