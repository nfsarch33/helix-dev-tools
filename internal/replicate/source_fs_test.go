package replicate

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// helper to create a temp Cursor-style skill catalogue.
//
// layout:
//
//	root/
//	  alpha/SKILL.md
//	  bravo/SKILL.md
//	  not-a-skill/    (no SKILL.md, must be ignored)
//	  .hidden/SKILL.md (ignored: dotfile)
//	  README.md       (file, not a dir; ignored)
func writeSkillCatalogue(t *testing.T, root string) {
	t.Helper()
	must := func(p string) {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("# skill\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	must(filepath.Join(root, "alpha", "SKILL.md"))
	must(filepath.Join(root, "bravo", "SKILL.md"))
	if err := os.MkdirAll(filepath.Join(root, "not-a-skill"), 0o755); err != nil {
		t.Fatal(err)
	}
	must(filepath.Join(root, ".hidden", "SKILL.md"))
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFSSource_SkillsBasic(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeSkillCatalogue(t, dir)
	s := NewFSSource("", "")
	got, err := s.Skills([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	want := []SkillEntry{
		{Name: "alpha", SourceDir: filepath.Join(dir, "alpha")},
		{Name: "bravo", SourceDir: filepath.Join(dir, "bravo")},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Skills mismatch:\nwant=%v\ngot =%v", want, got)
	}
}

func TestFSSource_SkillsCollisionAcrossRoots(t *testing.T) {
	t.Parallel()
	dirA := t.TempDir()
	dirB := t.TempDir()
	writeSkillCatalogue(t, dirA)
	writeSkillCatalogue(t, dirB)

	s := NewFSSource("", "")
	got, err := s.Skills([]string{dirA, dirB})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 unique skills (alpha, bravo), got %d", len(got))
	}
	collisions := s.SkillCollisions()
	if len(collisions) != 2 {
		t.Fatalf("expected 2 collisions from the second root, got %d", len(collisions))
	}
}

func TestFSSource_SkillsMissingRootIsNoop(t *testing.T) {
	t.Parallel()
	s := NewFSSource("", "")
	got, err := s.Skills([]string{"/nope/does/not/exist"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("missing root must be no-op, got %v", got)
	}
}

func TestFSSource_AgentsBasic(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, name := range []string{"go-architect.md", "memory-ops.md", "README.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := NewFSSource("", "")
	got, err := s.Agents(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []AgentEntry{
		{Name: "go-architect.md", SourceFile: filepath.Join(dir, "go-architect.md")},
		{Name: "memory-ops.md", SourceFile: filepath.Join(dir, "memory-ops.md")},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Agents mismatch:\nwant=%v\ngot =%v", want, got)
	}
}

func TestFSSource_HooksPathResolvesSymlink(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	real := filepath.Join(tmp, "real-hooks.json")
	link := filepath.Join(tmp, "alias-hooks.json")
	if err := os.WriteFile(real, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	s := NewFSSource(link, "")
	got := s.HooksPath()
	wantReal, _ := filepath.EvalSymlinks(real)
	if got != wantReal {
		t.Fatalf("HooksPath should resolve symlink: want=%q got=%q", wantReal, got)
	}
}

func TestFSSource_HooksPathMissing(t *testing.T) {
	t.Parallel()
	s := NewFSSource("/nope/hooks.json", "")
	if got := s.HooksPath(); got != "" {
		t.Fatalf("missing hooks file must yield empty string, got %q", got)
	}
}

func TestFSSource_MCPRawBasic(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mcp := filepath.Join(dir, "mcp.json")
	if err := os.WriteFile(mcp, []byte(`{"mcpServers":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewFSSource("", mcp)
	raw, err := s.MCPRaw()
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"mcpServers":{}}` {
		t.Fatalf("MCPRaw mismatch: %q", string(raw))
	}
}

func TestFSSource_MCPRawMissing(t *testing.T) {
	t.Parallel()
	s := NewFSSource("", "/nope/mcp.json")
	raw, err := s.MCPRaw()
	if err != nil {
		t.Fatalf("missing MCP file must NOT return error, got %v", err)
	}
	if len(raw) != 0 {
		t.Fatalf("missing MCP file must return empty bytes, got %v", raw)
	}
}
