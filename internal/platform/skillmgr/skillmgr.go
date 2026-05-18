// Package skillmgr implements cursor-tools skill install/list/remove.
// Skills live under ~/.cursor/skills/<name>/SKILL.md.
// Install clones a GitHub repo to a temp dir, runs skillvet scan,
// copies SKILL.md, and appends a pointer to the 00-index.
package skillmgr

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// SkillsDir is the canonical install directory, overridable in tests.
var SkillsDir = ""

func skillsDir() string {
	if SkillsDir != "" {
		return SkillsDir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cursor", "skills")
}

// IndexPath returns the 00-index SKILL.md pointer file.
func IndexPath() string {
	return filepath.Join(skillsDir(), "00-index", "SKILL.md")
}

// InstalledSkill describes a skill found on disk.
type InstalledSkill struct {
	Name        string
	SkillMDPath string
	InstalledAt string // from SKILL.md frontmatter "installed:" line if present
}

// Install clones repoURL into a temp dir, optionally runs skillvet scan
// (skipped when noVet is true), then copies the SKILL.md to the skills dir.
// repoURL may be a full URL or a "owner/repo" GitHub shorthand.
func Install(repoURL string, noVet bool) error {
	if repoURL == "" {
		return fmt.Errorf("repo URL is required")
	}
	if !strings.Contains(repoURL, "://") && !strings.HasPrefix(repoURL, "git@") {
		repoURL = "https://github.com/" + repoURL
	}

	name := repoName(repoURL)
	targetDir := filepath.Join(skillsDir(), name)
	if _, err := os.Stat(targetDir); err == nil {
		return fmt.Errorf("skill %q already installed at %s", name, targetDir)
	}

	tmpDir, err := os.MkdirTemp("", "skill-install-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := gitClone(repoURL, tmpDir); err != nil {
		return fmt.Errorf("clone %s: %w", repoURL, err)
	}

	skillMD := filepath.Join(tmpDir, "SKILL.md")
	if _, err := os.Stat(skillMD); err != nil {
		return fmt.Errorf("no SKILL.md found in %s", repoURL)
	}

	if !noVet {
		if err := skillvetScan(tmpDir); err != nil {
			return fmt.Errorf("skillvet scan failed: %w", err)
		}
	}

	if err := os.MkdirAll(targetDir, 0o750); err != nil {
		return fmt.Errorf("create skill dir: %w", err)
	}

	src, err := os.ReadFile(skillMD)
	if err != nil {
		return fmt.Errorf("read SKILL.md: %w", err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "SKILL.md"), src, 0o640); err != nil {
		return fmt.Errorf("write SKILL.md: %w", err)
	}

	return appendToIndex(name, repoURL)
}

// List returns all installed skills found under the skills directory.
func List() ([]InstalledSkill, error) {
	base := skillsDir()
	entries, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var skills []InstalledSkill
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "00-index" {
			continue
		}
		md := filepath.Join(base, e.Name(), "SKILL.md")
		if _, err := os.Stat(md); err != nil {
			continue
		}
		skills = append(skills, InstalledSkill{
			Name:        e.Name(),
			SkillMDPath: md,
			InstalledAt: extractInstalledAt(md),
		})
	}
	return skills, nil
}

// Remove deletes a skill directory and removes its entry from the 00-index.
func Remove(name string) error {
	if name == "" {
		return fmt.Errorf("skill name is required")
	}
	targetDir := filepath.Join(skillsDir(), name)
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		return fmt.Errorf("skill %q not installed", name)
	}
	if err := os.RemoveAll(targetDir); err != nil {
		return fmt.Errorf("remove skill dir: %w", err)
	}
	return removeFromIndex(name)
}

// -- helpers --

func repoName(repoURL string) string {
	parts := strings.Split(strings.TrimSuffix(repoURL, ".git"), "/")
	if len(parts) == 0 {
		return "unknown"
	}
	return parts[len(parts)-1]
}

func gitClone(repoURL, dest string) error {
	cmd := exec.Command("git", "clone", "--depth=1", repoURL, dest)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func skillvetScan(dir string) error {
	cmd := exec.Command("skillvet", "scan", dir)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if isNotFound(err) {
			// skillvet not installed; warn but don't block
			fmt.Fprintf(os.Stderr, "warning: skillvet not found, skipping security scan\n")
			return nil
		}
		return err
	}
	return nil
}

func isNotFound(err error) bool {
	return strings.Contains(err.Error(), "executable file not found") ||
		strings.Contains(err.Error(), "no such file")
}

func appendToIndex(name, source string) error {
	indexPath := IndexPath()
	if err := os.MkdirAll(filepath.Dir(indexPath), 0o750); err != nil {
		return err
	}

	f, err := os.OpenFile(indexPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	defer f.Close()

	ts := time.Now().UTC().Format("2006-01-02")
	_, err = fmt.Fprintf(f, "- [%s](%s/SKILL.md) -- installed %s from %s\n", name, name, ts, source)
	return err
}

func removeFromIndex(name string) error {
	indexPath := IndexPath()
	raw, err := os.ReadFile(indexPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	var kept []string
	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	prefix := fmt.Sprintf("- [%s]", name)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, prefix) {
			kept = append(kept, line)
		}
	}

	return os.WriteFile(indexPath, []byte(strings.Join(kept, "\n")+"\n"), 0o640)
}

func extractInstalledAt(mdPath string) string {
	f, err := os.Open(mdPath)
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "installed:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "installed:"))
		}
	}
	return ""
}
