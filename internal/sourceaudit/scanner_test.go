package sourceaudit_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/sourceaudit"
)

func TestScanSkillWithSource(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "test-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `---
name: test-skill
description: "Test skill"
---
# Test
<!-- Source: https://github.com/example/repo -->
<!-- Evolution: 2026-03-09 | reason: test -->
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := sourceaudit.ScanSkill(skillDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.Name != "test-skill" {
		t.Errorf("expected name 'test-skill', got '%s'", result.Name)
	}
	if len(result.Sources) != 1 {
		t.Errorf("expected 1 source, got %d", len(result.Sources))
	}
	if result.Sources[0] != "https://github.com/example/repo" {
		t.Errorf("expected github URL, got '%s'", result.Sources[0])
	}
	if result.HasSources != true {
		t.Error("expected HasSources to be true")
	}
}

func TestScanSkillWithoutSource(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "no-source-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `---
name: no-source-skill
description: "Skill without source"
---
# No Source
Some content without any source markers.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := sourceaudit.ScanSkill(skillDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.HasSources != false {
		t.Error("expected HasSources to be false")
	}
	if len(result.Sources) != 0 {
		t.Errorf("expected 0 sources, got %d", len(result.Sources))
	}
}

func TestScanSkillMultipleSources(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "multi-source")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `---
name: multi-source
description: "Multi source skill"
---
# Multi
<!-- Source: https://github.com/org/repo1 -->
<!-- Source: https://arxiv.org/abs/1234.5678 -->
<!-- Source: https://skills.sh/author/skills/name -->
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := sourceaudit.ScanSkill(skillDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Sources) != 3 {
		t.Errorf("expected 3 sources, got %d", len(result.Sources))
	}
	if !result.HasSources {
		t.Error("expected HasSources to be true")
	}
}

func TestScanAllSkills(t *testing.T) {
	dir := t.TempDir()

	for _, name := range []string{"skill-a", "skill-b"} {
		skillDir := filepath.Join(dir, name)
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			t.Fatal(err)
		}
		content := "---\nname: " + name + "\n---\n# " + name + "\n"
		if name == "skill-a" {
			content += "<!-- Source: https://github.com/example/a -->\n"
		}
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	results, err := sourceaudit.ScanAll(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	withSources := 0
	withoutSources := 0
	for _, r := range results {
		if r.HasSources {
			withSources++
		} else {
			withoutSources++
		}
	}
	if withSources != 1 {
		t.Errorf("expected 1 with sources, got %d", withSources)
	}
	if withoutSources != 1 {
		t.Errorf("expected 1 without sources, got %d", withoutSources)
	}
}
