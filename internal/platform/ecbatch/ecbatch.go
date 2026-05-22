package ecbatch

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// BatchStatus represents the state of one EC programme batch worktree.
type BatchStatus struct {
	BatchNum      int
	WorktreePath  string
	Branch        string
	CommitSHA     string
	Packages      []string
	TestsPassed   bool
	Pushed        bool
}

// Scanner scans a set of worktree paths and returns batch statuses.
type Scanner struct {
	WorktreeRoot string
	MaxBatches   int
}

// ScannerOption configures Scanner behavior
type ScannerOption func(*Scanner)

// WithWorktreeRoot sets the root directory to scan for worktrees
func WithWorktreeRoot(root string) ScannerOption {
	return func(s *Scanner) {
		s.WorktreeRoot = root
	}
}

// WithMaxBatches sets the maximum number of batches to scan
func WithMaxBatches(n int) ScannerOption {
	return func(s *Scanner) {
		s.MaxBatches = n
	}
}

// NewScanner creates a Scanner with default or custom options
func NewScanner(opts ...ScannerOption) *Scanner {
	s := &Scanner{
		WorktreeRoot: "/tmp",
		MaxBatches:   20,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Scan returns status for all ec-batchN worktrees found
func (s *Scanner) Scan() ([]BatchStatus, error) {
	var batches []BatchStatus

	for i := 1; i <= s.MaxBatches; i++ {
		batchPath := filepath.Join(s.WorktreeRoot, fmt.Sprintf("ec-batch%d", i))

		// Check if worktree directory exists
		if _, err := os.Stat(batchPath); os.IsNotExist(err) {
			continue
		}

		// Attempt to read branch and commit info
		status, err := s.scanBatchStatus(batchPath)
		if err != nil {
			// Skip if cannot read status, but don't stop entire scan
			continue
		}

		batches = append(batches, status)
	}

	return batches, nil
}

// scanBatchStatus reads the git worktree status for a specific batch
func (s *Scanner) scanBatchStatus(batchPath string) (BatchStatus, error) {
	status := BatchStatus{
		WorktreePath: batchPath,
	}

	// Extract batch number from path
	matches := strings.Split(filepath.Base(batchPath), "ec-batch")
	if len(matches) > 1 {
		batchNum, _ := strconv.Atoi(matches[1])
		status.BatchNum = batchNum
	}

	// Find the .git file indicating the real git directory
	gitFilePath := filepath.Join(batchPath, ".git")
	gitFileBytes, err := os.ReadFile(gitFilePath)
	if err != nil {
		return status, fmt.Errorf("cannot read .git file: %v", err)
	}

	// Extract the real git worktree path
	gitDirMatch := strings.TrimPrefix(string(gitFileBytes), "gitdir: ")
	gitDirPath := strings.TrimSpace(gitDirMatch)

	// Read HEAD to get branch or detached HEAD state
	headPath := filepath.Join(gitDirPath, "HEAD")
	headBytes, err := os.ReadFile(headPath)
	if err != nil {
		return status, fmt.Errorf("cannot read HEAD: %v", err)
	}

	headContent := string(headBytes)
	if strings.HasPrefix(headContent, "ref: refs/heads/") {
		status.Branch = strings.TrimPrefix(headContent, "ref: refs/heads/")
		status.Branch = strings.TrimSpace(status.Branch)
	} else {
		status.Branch = "(detached HEAD)"
		status.CommitSHA = strings.TrimSpace(headContent)
	}

	status.Packages = scanGoPackages(batchPath)
	return status, nil
}

// Summary returns aggregate stats for batch statuses
func Summarize(batches []BatchStatus) Summary {
	summary := Summary{
		Total:   len(batches),
		Pushed:  0,
		Pending: len(batches),
		Batches: batches,
	}

	for _, batch := range batches {
		if batch.Pushed {
			summary.Pushed++
			summary.Pending--
		}
	}

	return summary
}

// scanGoPackages walks batchPath/internal/ looking for directories
// that contain at least one .go file and returns their relative paths.
func scanGoPackages(batchPath string) []string {
	internalDir := filepath.Join(batchPath, "internal")
	info, err := os.Stat(internalDir)
	if err != nil || !info.IsDir() {
		return nil
	}

	var packages []string
	filepath.WalkDir(internalDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		entries, readErr := os.ReadDir(path)
		if readErr != nil {
			return nil
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
				rel, relErr := filepath.Rel(batchPath, path)
				if relErr == nil {
					packages = append(packages, rel)
				}
				break
			}
		}
		return nil
	})
	return packages
}

// Summary provides aggregate statistics about batch statuses
type Summary struct {
	Total   int
	Pushed  int
	Pending int
	Batches []BatchStatus
}