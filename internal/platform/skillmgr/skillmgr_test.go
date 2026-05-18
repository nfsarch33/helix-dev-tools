package skillmgr

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withSkillsDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	SkillsDir = dir
	t.Cleanup(func() { SkillsDir = "" })
	return dir
}

func makeSkill(t *testing.T, base, name, content string) {
	t.Helper()
	skillDir := filepath.Join(base, name)
	if err := os.MkdirAll(skillDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o640); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestListEmpty(t *testing.T) {
	withSkillsDir(t)
	skills, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(skills))
	}
}

func TestListFindsInstalledSkills(t *testing.T) {
	base := withSkillsDir(t)
	makeSkill(t, base, "my-skill", "# My Skill\ninstalled: 2026-05-19\n")
	makeSkill(t, base, "other-skill", "# Other Skill\n")

	skills, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}
}

func TestListSkipsDirsWithoutSkillMD(t *testing.T) {
	base := withSkillsDir(t)
	makeSkill(t, base, "valid", "content")
	// create a dir without SKILL.md
	os.MkdirAll(filepath.Join(base, "empty-dir"), 0o750)

	skills, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(skills) != 1 {
		t.Errorf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Name != "valid" {
		t.Errorf("expected 'valid', got %q", skills[0].Name)
	}
}

func TestListSkips00Index(t *testing.T) {
	base := withSkillsDir(t)
	makeSkill(t, base, "real-skill", "content")
	// 00-index should be excluded even with SKILL.md
	indexDir := filepath.Join(base, "00-index")
	os.MkdirAll(indexDir, 0o750)
	os.WriteFile(filepath.Join(indexDir, "SKILL.md"), []byte("index"), 0o640)

	skills, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(skills) != 1 || skills[0].Name != "real-skill" {
		t.Errorf("00-index should be excluded; got %v", skills)
	}
}

func TestRemoveDeletesSkillDir(t *testing.T) {
	base := withSkillsDir(t)
	makeSkill(t, base, "remove-me", "content")

	if err := Remove("remove-me"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if _, err := os.Stat(filepath.Join(base, "remove-me")); !os.IsNotExist(err) {
		t.Error("skill dir should be deleted")
	}
}

func TestRemoveNotInstalledReturnsError(t *testing.T) {
	withSkillsDir(t)
	err := Remove("nonexistent")
	if err == nil {
		t.Fatal("expected error removing nonexistent skill")
	}
	if !strings.Contains(err.Error(), "not installed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRemoveEmptyNameReturnsError(t *testing.T) {
	withSkillsDir(t)
	err := Remove("")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestRemoveUpdatesIndex(t *testing.T) {
	base := withSkillsDir(t)
	makeSkill(t, base, "to-remove", "content")
	makeSkill(t, base, "to-keep", "content")

	// Write a synthetic index
	indexPath := IndexPath()
	os.MkdirAll(filepath.Dir(indexPath), 0o750)
	index := "- [to-remove](to-remove/SKILL.md) -- installed 2026-05-19 from http://example.com\n" +
		"- [to-keep](to-keep/SKILL.md) -- installed 2026-05-19 from http://example.com\n"
	os.WriteFile(indexPath, []byte(index), 0o640)

	if err := Remove("to-remove"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	raw, _ := os.ReadFile(indexPath)
	contents := string(raw)
	if strings.Contains(contents, "to-remove") {
		t.Error("index still contains to-remove entry")
	}
	if !strings.Contains(contents, "to-keep") {
		t.Error("index lost to-keep entry")
	}
}

func TestRepoName(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"https://github.com/owner/my-skill.git", "my-skill"},
		{"https://github.com/owner/my-skill", "my-skill"},
		{"owner/my-skill", "my-skill"},
		{"git@github.com:owner/my-skill.git", "my-skill"},
	}
	for _, c := range cases {
		got := repoName(c.url)
		if got != c.want {
			t.Errorf("repoName(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}

func TestInstallAlreadyInstalledReturnsError(t *testing.T) {
	base := withSkillsDir(t)
	makeSkill(t, base, "existing", "content")

	// passing a URL whose base name resolves to "existing"
	err := Install("https://github.com/owner/existing", true)
	if err == nil {
		t.Fatal("expected error for already-installed skill")
	}
	if !strings.Contains(err.Error(), "already installed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInstallEmptyURLReturnsError(t *testing.T) {
	withSkillsDir(t)
	err := Install("", true)
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}
