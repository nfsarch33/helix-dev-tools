package noshellleaksync

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMirrorRepoPaths_FourteenAndDeterministic locks the mirror target list
// at exactly 14 personal repos and ensures the order is deterministic. If a
// repo is added or removed, the matching v299 docs and SOP must be updated
// in the same change -- this test exists to force that synchronisation.
func TestMirrorRepoPaths_FourteenAndDeterministic(t *testing.T) {
	got := MirrorRepoPaths()
	if len(got) != 14 {
		t.Fatalf("want 14 mirror targets, got %d: %v", len(got), got)
	}
	want := []string{
		"agentic-ai-research",
		"ai-agent-business-stack",
		"Code/global-kb",
		"Code/pdf-mcp-server",
		"Code/secure-auth-platform",
		"cursor-tools",
		"hermes-agent",
		"helixon",
		"helixon-mcp",
		"helixon-ops",
		"linkedin-mcp-server",
		"memo",
		"openclaw-mission-control",
		"upwork-mcp",
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("mirror[%d]: got %q, want %q", i, got[i], w)
		}
	}
}

// TestSyncer_DetectsAndResyncsDrift covers the happy path: canonical exists,
// mirror exists but differs -> action is ActionResynced and the file on
// disk now matches the canonical bytes.
func TestSyncer_DetectsAndResyncsDrift(t *testing.T) {
	homeDir := t.TempDir()

	canonicalRel := filepath.Join("global-kb", "rules", "no-shell-leak.mdc")
	mustWriteFile(t, filepath.Join(homeDir, canonicalRel), "canonical-content-v299\n")

	mirrorRel := filepath.Join(".cursor", "rules", "no-shell-leak.mdc")
	repos := []string{"repo-a", "repo-b", "repo-c"}
	for _, r := range repos {
		mustWriteFile(t, filepath.Join(homeDir, r, mirrorRel), "stale-content-old\n")
	}

	syncer := &Syncer{
		HomeDir:      homeDir,
		CanonicalRel: canonicalRel,
		MirrorRel:    mirrorRel,
		Repos:        repos,
		WriteOnDrift: true,
	}

	results, err := syncer.Run()
	if err != nil {
		t.Fatalf("syncer.Run: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("want 3 results, got %d", len(results))
	}
	for _, res := range results {
		if res.Action != ActionResynced {
			t.Errorf("repo %s: want %s, got %s (note=%s)",
				res.RepoPath, ActionResynced, res.Action, res.Note)
		}
		got, err := os.ReadFile(res.MirrorPath)
		if err != nil {
			t.Errorf("read mirror %s: %v", res.MirrorPath, err)
			continue
		}
		if string(got) != "canonical-content-v299\n" {
			t.Errorf("mirror %s: got %q, want canonical bytes", res.MirrorPath, got)
		}
	}
}

// TestSyncer_NoOpWhenInSync covers the steady-state path: every mirror
// already matches the canonical SHA -> no writes occur and every result
// is ActionInSync.
func TestSyncer_NoOpWhenInSync(t *testing.T) {
	homeDir := t.TempDir()

	canonicalRel := filepath.Join("global-kb", "rules", "no-shell-leak.mdc")
	canonicalContent := "canonical-content-v299\n"
	mustWriteFile(t, filepath.Join(homeDir, canonicalRel), canonicalContent)

	mirrorRel := filepath.Join(".cursor", "rules", "no-shell-leak.mdc")
	repos := []string{"repo-a", "repo-b"}
	for _, r := range repos {
		mustWriteFile(t, filepath.Join(homeDir, r, mirrorRel), canonicalContent)
	}

	syncer := &Syncer{
		HomeDir:      homeDir,
		CanonicalRel: canonicalRel,
		MirrorRel:    mirrorRel,
		Repos:        repos,
		WriteOnDrift: true,
	}

	results, err := syncer.Run()
	if err != nil {
		t.Fatalf("syncer.Run: %v", err)
	}
	for _, res := range results {
		if res.Action != ActionInSync {
			t.Errorf("repo %s: want %s, got %s", res.RepoPath, ActionInSync, res.Action)
		}
	}
}

// TestSyncer_SkipsRepoMissing ensures that absent repo directories are
// skipped (ActionRepoMissing) rather than treated as errors. WSL-only
// repos may not exist on the Macbook side and vice-versa.
func TestSyncer_SkipsRepoMissing(t *testing.T) {
	homeDir := t.TempDir()

	canonicalRel := filepath.Join("global-kb", "rules", "no-shell-leak.mdc")
	mustWriteFile(t, filepath.Join(homeDir, canonicalRel), "canonical\n")

	syncer := &Syncer{
		HomeDir:      homeDir,
		CanonicalRel: canonicalRel,
		MirrorRel:    filepath.Join(".cursor", "rules", "no-shell-leak.mdc"),
		Repos:        []string{"never-cloned-repo"},
		WriteOnDrift: true,
	}

	results, err := syncer.Run()
	if err != nil {
		t.Fatalf("syncer.Run: %v", err)
	}
	if len(results) != 1 || results[0].Action != ActionRepoMissing {
		t.Fatalf("want 1 result with action %s, got %+v", ActionRepoMissing, results)
	}
}

// TestSyncer_SkipsMirrorMissing ensures that repos which exist but have no
// .cursor/rules/no-shell-leak.mdc file are skipped rather than auto-created.
// Opting out of the rule mirror is a valid choice (e.g. public mirror repos
// not in the canonical 14-repo list).
func TestSyncer_SkipsMirrorMissing(t *testing.T) {
	homeDir := t.TempDir()

	canonicalRel := filepath.Join("global-kb", "rules", "no-shell-leak.mdc")
	mustWriteFile(t, filepath.Join(homeDir, canonicalRel), "canonical\n")

	repoDir := filepath.Join(homeDir, "repo-no-cursor-rules")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	syncer := &Syncer{
		HomeDir:      homeDir,
		CanonicalRel: canonicalRel,
		MirrorRel:    filepath.Join(".cursor", "rules", "no-shell-leak.mdc"),
		Repos:        []string{"repo-no-cursor-rules"},
		WriteOnDrift: true,
	}

	results, err := syncer.Run()
	if err != nil {
		t.Fatalf("syncer.Run: %v", err)
	}
	if len(results) != 1 || results[0].Action != ActionMirrorMissing {
		t.Fatalf("want 1 result with action %s, got %+v", ActionMirrorMissing, results)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
