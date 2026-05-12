package branchcleanup

import (
	"fmt"
	"strings"
	"testing"
)

type mockRunner struct {
	outputs map[string]string
	calls   []string
}

func (m *mockRunner) Run(args ...string) (string, error) {
	key := strings.Join(args, " ")
	m.calls = append(m.calls, key)
	if out, ok := m.outputs[key]; ok {
		return out, nil
	}
	return "", fmt.Errorf("unexpected call: %s", key)
}

func TestCleanup_DeletesLocalMergedBranch(t *testing.T) {
	runner := &mockRunner{outputs: map[string]string{
		"branch --show-current":          "main\n",
		"branch --merged main":           "* main\n  feat/old\n",
		"branch -r --merged origin/main": "  origin/main\n  origin/HEAD -> origin/main\n",
		"branch -d feat/old":             "Deleted branch feat/old.\n",
	}}

	result := Cleanup(runner, "/tmp/repo", false)

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if len(result.LocalDeleted) != 1 || result.LocalDeleted[0] != "feat/old" {
		t.Errorf("expected LocalDeleted=[feat/old], got %v", result.LocalDeleted)
	}
}

func TestCleanup_PreservesMainBranch(t *testing.T) {
	runner := &mockRunner{outputs: map[string]string{
		"branch --show-current":          "feat/wip\n",
		"branch --merged main":           "* feat/wip\n  main\n  master\n  feat/done\n",
		"branch -r --merged origin/main": "  origin/main\n  origin/HEAD -> origin/main\n",
		"branch -d feat/done":            "Deleted branch feat/done.\n",
	}}

	result := Cleanup(runner, "/tmp/repo", false)

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	found := false
	for _, s := range result.Skipped {
		if s == "main" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'main' in Skipped, got %v", result.Skipped)
	}
	for _, d := range result.LocalDeleted {
		if d == "main" || d == "master" {
			t.Errorf("protected branch %q should not be deleted", d)
		}
	}
}

func TestCleanup_PreservesProtectedBranchCheckedOutInAnotherWorktree(t *testing.T) {
	runner := &mockRunner{outputs: map[string]string{
		"branch --show-current":          "feat/wip\n",
		"branch --merged main":           "* feat/wip\n+ main\n  feat/done\n",
		"branch -r --merged origin/main": "  origin/main\n",
		"branch -d feat/done":            "Deleted branch feat/done.\n",
	}}

	result := Cleanup(runner, "/repo", false)

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	for _, d := range result.LocalDeleted {
		if d == "+ main" || d == "main" {
			t.Fatalf("protected branch should not be deleted: %v", result.LocalDeleted)
		}
	}
	found := false
	for _, s := range result.Skipped {
		if s == "main" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected normalized main in skipped list, got %v", result.Skipped)
	}
}

func TestCleanup_DryRunDoesNotDelete(t *testing.T) {
	runner := &mockRunner{outputs: map[string]string{
		"branch --show-current":          "main\n",
		"branch --merged main":           "* main\n  feat/old\n  feat/stale\n",
		"branch -r --merged origin/main": "  origin/main\n  origin/feat/old\n",
	}}

	result := Cleanup(runner, "/tmp/repo", true)

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if !result.DryRun {
		t.Error("expected DryRun=true")
	}
	if len(result.LocalDeleted) != 2 {
		t.Errorf("expected 2 local candidates, got %v", result.LocalDeleted)
	}
	if len(result.RemoteDeleted) != 1 || result.RemoteDeleted[0] != "feat/old" {
		t.Errorf("expected RemoteDeleted=[feat/old], got %v", result.RemoteDeleted)
	}
	for _, call := range runner.calls {
		if strings.Contains(call, "branch -d") || strings.Contains(call, "push origin --delete") {
			t.Errorf("dry-run should not invoke delete commands, but called: %s", call)
		}
	}
}

func TestCleanupWithOptions_PrunesAndDeletesOnlyLocalMergedBranches(t *testing.T) {
	runner := &mockRunner{outputs: map[string]string{
		"fetch --prune origin":           "",
		"branch --show-current":          "main\n",
		"branch --merged main":           "* main\n  feat/done\n",
		"branch -r --merged origin/main": "  origin/main\n  origin/feat/done\n",
		"branch -d feat/done":            "Deleted branch feat/done.\n",
	}}

	result := CleanupWithOptions(runner, "/repo", Options{
		PruneStaleTracking: true,
		DeleteLocalMerged:  true,
		DeleteRemoteMerged: false,
	})

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if !containsString(runner.calls, "fetch --prune origin") {
		t.Fatalf("expected stale tracking prune, calls=%v", runner.calls)
	}
	if len(result.LocalDeleted) != 1 || result.LocalDeleted[0] != "feat/done" {
		t.Fatalf("local deleted = %v, want [feat/done]", result.LocalDeleted)
	}
	if len(result.RemoteDeleted) != 0 {
		t.Fatalf("remote branches should not be deleted: %v", result.RemoteDeleted)
	}
}

func TestCleanup_SkipsCurrentBranch(t *testing.T) {
	runner := &mockRunner{outputs: map[string]string{
		"branch --show-current":          "feat/wip\n",
		"branch --merged main":           "* feat/wip\n  main\n  feat/done\n",
		"branch -r --merged origin/main": "  origin/main\n  origin/HEAD -> origin/main\n",
		"branch -d feat/done":            "Deleted branch feat/done.\n",
	}}

	result := Cleanup(runner, "/tmp/repo", false)

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	found := false
	for _, s := range result.Skipped {
		if s == "feat/wip" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected current branch 'feat/wip' in Skipped, got %v", result.Skipped)
	}
}

func TestCleanupFleet_RunsEveryRepo(t *testing.T) {
	runners := map[string]*mockRunner{
		"/repo/a": {outputs: map[string]string{
			"branch --show-current":          "main\n",
			"branch --merged main":           "* main\n  feat/a\n",
			"branch -r --merged origin/main": "  origin/main\n",
		}},
		"/repo/b": {outputs: map[string]string{
			"branch --show-current":          "main\n",
			"branch --merged main":           "* main\n  feat/b\n",
			"branch -r --merged origin/main": "  origin/main\n  origin/feat/b\n",
		}},
	}

	results := CleanupFleet([]Repo{
		{Alias: "a", Path: "/repo/a"},
		{Alias: "b", Path: "/repo/b"},
	}, func(path string) GitRunner {
		return runners[path]
	}, true)

	if len(results) != 2 {
		t.Fatalf("expected 2 fleet results, got %d", len(results))
	}
	if results[0].Alias != "a" || results[1].Alias != "b" {
		t.Fatalf("aliases not preserved: %#v", results)
	}
}

func TestCleanupWithOptions_DetectsSquashMergedBranches(t *testing.T) {
	runner := &mockRunner{outputs: map[string]string{
		"branch --show-current":                                         "main\n",
		"branch --merged main":                                          "* main\n",
		"branch -r --merged origin/main":                                "  origin/main\n",
		"branch --no-merged main":                                       "  feat/squashed\n  feat/active\n",
		"log --oneline --cherry-pick --right-only main...feat/squashed": "",
		"log --oneline --cherry-pick --right-only main...feat/active":   "abc1234 still working\n",
		"branch -d feat/squashed":                                       "Deleted branch feat/squashed.\n",
	}}

	result := CleanupWithOptions(runner, "/repo", Options{
		DeleteLocalMerged:  true,
		DeleteRemoteMerged: true,
		DetectSquashMerged: true,
	})

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	found := false
	for _, d := range result.LocalDeleted {
		if d == "feat/squashed" {
			found = true
		}
		if d == "feat/active" {
			t.Error("feat/active should NOT be deleted (has unmerged commits)")
		}
	}
	if !found {
		t.Errorf("expected squash-merged feat/squashed in LocalDeleted, got %v", result.LocalDeleted)
	}
}

func TestCleanupWithOptions_SquashMergedDryRun(t *testing.T) {
	runner := &mockRunner{outputs: map[string]string{
		"branch --show-current":                                         "main\n",
		"branch --merged main":                                          "* main\n",
		"branch -r --merged origin/main":                                "  origin/main\n",
		"branch --no-merged main":                                       "  feat/squashed\n",
		"log --oneline --cherry-pick --right-only main...feat/squashed": "",
	}}

	result := CleanupWithOptions(runner, "/repo", Options{
		DryRun:             true,
		DeleteLocalMerged:  true,
		DetectSquashMerged: true,
	})

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if len(result.LocalDeleted) != 1 || result.LocalDeleted[0] != "feat/squashed" {
		t.Errorf("expected [feat/squashed] in dry-run candidates, got %v", result.LocalDeleted)
	}
	for _, call := range runner.calls {
		if strings.Contains(call, "branch -d") {
			t.Errorf("dry-run should not invoke delete, but called: %s", call)
		}
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
