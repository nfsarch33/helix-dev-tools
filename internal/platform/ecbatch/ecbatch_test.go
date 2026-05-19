package ecbatch

import (
	"os"
	"path/filepath"
	"testing"
)

func createTestBatchWorktree(t *testing.T, root, batchName, branch string) {
	t.Helper()
	batchPath := filepath.Join(root, batchName)
	err := os.MkdirAll(batchPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create batch directory: %v", err)
	}

	// Create .git file pointing to a fake git worktree
	gitWorktreePath := filepath.Join(root, ".git", "worktrees", batchName)
	err = os.MkdirAll(gitWorktreePath, 0755)
	if err != nil {
		t.Fatalf("Failed to create git worktree path: %v", err)
	}

	gitFilePath := filepath.Join(batchPath, ".git")
	gitFileContent := []byte("gitdir: " + filepath.Join(root, ".git", "worktrees", batchName))
	err = os.WriteFile(gitFilePath, gitFileContent, 0644)
	if err != nil {
		t.Fatalf("Failed to write .git file: %v", err)
	}

	// Write HEAD file with branch
	headPath := filepath.Join(gitWorktreePath, "HEAD")
	var headContent []byte
	if branch == "" {
		headContent = []byte("0a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p")
	} else {
		headContent = []byte("ref: refs/heads/" + branch)
	}
	err = os.WriteFile(headPath, headContent, 0644)
	if err != nil {
		t.Fatalf("Failed to write HEAD file: %v", err)
	}
}

func TestScannerDefaults(t *testing.T) {
	t.Parallel()
	scanner := NewScanner()

	if scanner.WorktreeRoot != "/tmp" {
		t.Errorf("Default WorktreeRoot should be /tmp, got %s", scanner.WorktreeRoot)
	}
	if scanner.MaxBatches != 20 {
		t.Errorf("Default MaxBatches should be 20, got %d", scanner.MaxBatches)
	}
}

func TestScannerOptions(t *testing.T) {
	t.Parallel()
	scanner := NewScanner(
		WithWorktreeRoot("/custom/path"),
		WithMaxBatches(10),
	)

	if scanner.WorktreeRoot != "/custom/path" {
		t.Errorf("WorktreeRoot not set correctly, got %s", scanner.WorktreeRoot)
	}
	if scanner.MaxBatches != 10 {
		t.Errorf("MaxBatches not set correctly, got %d", scanner.MaxBatches)
	}
}

func TestScanNoWorktrees(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	scanner := NewScanner(WithWorktreeRoot(tmpDir))

	batches, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan should not return error on empty dir: %v", err)
	}
	if len(batches) != 0 {
		t.Errorf("Expected no batches in empty directory, got %d", len(batches))
	}
}

func TestScanMultipleBatches(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	scanner := NewScanner(WithWorktreeRoot(tmpDir), WithMaxBatches(3))

	// Create some test worktrees
	createTestBatchWorktree(t, tmpDir, "ec-batch1", "main")
	createTestBatchWorktree(t, tmpDir, "ec-batch2", "feature-branch")
	// Skip a batch to test gap handling
	createTestBatchWorktree(t, tmpDir, "ec-batch3", "")

	batches, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(batches) != 3 {
		t.Errorf("Expected 3 batches, got %d", len(batches))
	}

	// Validate branch names
	expectedBranches := []string{"main", "feature-branch", "(detached HEAD)"}
	for i, batch := range batches {
		if batch.BatchNum != i+1 {
			t.Errorf("Batch number mismatch: expected %d, got %d", i+1, batch.BatchNum)
		}
		if batch.Branch != expectedBranches[i] {
			t.Errorf("Unexpected branch name at index %d: got %s", i, batch.Branch)
		}
	}
}

func TestSummarize(t *testing.T) {
	t.Parallel()
	batches := []BatchStatus{
		{BatchNum: 1, Pushed: false},
		{BatchNum: 2, Pushed: true},
		{BatchNum: 3, Pushed: false},
	}

	summary := Summarize(batches)

	if summary.Total != 3 {
		t.Errorf("Total batches incorrect: got %d, want 3", summary.Total)
	}
	if summary.Pushed != 1 {
		t.Errorf("Pushed count incorrect: got %d, want 1", summary.Pushed)
	}
	if summary.Pending != 2 {
		t.Errorf("Pending count incorrect: got %d, want 2", summary.Pending)
	}
}

func TestBatchNumberExtraction(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	scanner := NewScanner(WithWorktreeRoot(tmpDir), WithMaxBatches(50))

	createTestBatchWorktree(t, tmpDir, "ec-batch42", "test-branch")

	batches, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(batches) != 1 {
		t.Fatalf("Expected 1 batch, got %d", len(batches))
	}

	if batches[0].BatchNum != 42 {
		t.Errorf("Batch number incorrect: got %d, want 42", batches[0].BatchNum)
	}
}