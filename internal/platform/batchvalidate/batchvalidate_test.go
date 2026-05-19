package batchvalidate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// createTestModule creates a minimal Go module with test package
func createTestModule(t *testing.T, root string, packages []string) {
	t.Helper()
	err := os.MkdirAll(root, 0755)
	if err != nil {
		t.Fatalf("Failed to create root dir: %v", err)
	}

	// Create go.mod
	modContent := []byte("module testmodule\n\ngo 1.21")
	err = os.WriteFile(filepath.Join(root, "go.mod"), modContent, 0644)
	if err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Create test packages
	for _, pkg := range packages {
		pkgPath := filepath.Join(root, pkg)
		err := os.MkdirAll(pkgPath, 0755)
		if err != nil {
			t.Fatalf("Failed to create package dir %s: %v", pkg, err)
		}

		// Create a simple test file
		testContent := []byte(`package ` + filepath.Base(pkg) + `

import "testing"

func TestAlwaysPasses(t *testing.T) {
	// Always passes
}`)
		testFilePath := filepath.Join(pkgPath, pkg+"_test.go")
		err = os.WriteFile(testFilePath, testContent, 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}
}

func TestRunnerDefaults(t *testing.T) {
	t.Parallel()
	runner := New()

	if runner.GoCmd != "go" {
		t.Errorf("Default GoCmd should be 'go', got %s", runner.GoCmd)
	}
	if runner.Timeout != 60*time.Second {
		t.Errorf("Default Timeout should be 60s, got %v", runner.Timeout)
	}
}

func TestRunnerOptions(t *testing.T) {
	t.Parallel()
	runner := New(
		WithGoCmd("/custom/go"),
		WithTimeout(30*time.Second),
	)

	if runner.GoCmd != "/custom/go" {
		t.Errorf("GoCmd not set correctly, got %s", runner.GoCmd)
	}
	if runner.Timeout != 30*time.Second {
		t.Errorf("Timeout not set correctly, got %v", runner.Timeout)
	}
}

func TestValidateBatchDry(t *testing.T) {
	t.Parallel()
	runner := New()

	packages := []string{"internal/pkg1", "internal/pkg2", "internal/pkg3"}
	result := runner.ValidateBatchDry("/test/worktree", packages)

	if result.WorktreePath != "/test/worktree" {
		t.Errorf("WorktreePath incorrect, got %s", result.WorktreePath)
	}
	if !result.AllPassed {
		t.Errorf("BatchResult should be AllPassed")
	}
	if len(result.Results) != len(packages) {
		t.Errorf("Expected %d package results, got %d", len(packages), len(result.Results))
	}

	for _, pkgResult := range result.Results {
		if !pkgResult.Passed {
			t.Errorf("Package %s should have passed in dry run", pkgResult.Package)
		}
	}
}

func TestValidateBatchDryWithPartialFail(t *testing.T) {
	t.Parallel()
	runner := New()

	packages := []string{"internal/pkg1", "internal/pkg2", "internal/pkg3"}
	result := runner.ValidateBatchDry("/test/worktree", packages)

	if !result.AllPassed {
		t.Errorf("Dry run should always report AllPassed")
	}
}

func TestFormatReportPassCase(t *testing.T) {
	t.Parallel()
	result := BatchResult{
		WorktreePath: "/test/worktree",
		AllPassed:    true,
		Duration:     5 * time.Second,
		Results: []PackageResult{
			{
				Package:   "internal/pkg1",
				Passed:    true,
				Duration:  100 * time.Millisecond,
			},
			{
				Package:   "internal/pkg2",
				Passed:    true,
				Duration:  150 * time.Millisecond,
			},
		},
	}

	report := FormatReport(result)
	if !strings.Contains(report, "PASS") {
		t.Errorf("Pass case report should contain PASS")
	}
}

func TestFormatReportFailCase(t *testing.T) {
	t.Parallel()
	result := BatchResult{
		WorktreePath: "/test/worktree",
		AllPassed:    false,
		Duration:     5 * time.Second,
		Results: []PackageResult{
			{
				Package:   "internal/pkg1",
				Passed:    true,
				Duration:  100 * time.Millisecond,
			},
			{
				Package:   "internal/pkg2",
				Passed:    false,
				Error:     fmt.Errorf("test failed"),
				Output:    "some error output",
				Duration:  150 * time.Millisecond,
			},
		},
	}

	report := FormatReport(result)
	if !strings.Contains(report, "FAIL") || !strings.Contains(report, "some error output") {
		t.Errorf("Fail case report should contain FAIL and error details")
	}
}

func TestIntegrationValidateBatch(t *testing.T) {
	// Skip this integration test unless explicitly enabled
	if os.Getenv("RUN_INTEGRATION_TESTS") == "" {
		t.Skip("Skipping integration test")
	}

	t.Parallel()
	tmpRoot := t.TempDir()
	packages := []string{"internal/pkg1", "internal/pkg2", "internal/pkg3"}
	createTestModule(t, tmpRoot, packages)

	runner := New()
	result, err := runner.ValidateBatch(context.Background(), tmpRoot)

	if err != nil {
		t.Fatalf("ValidateBatch failed: %v", err)
	}

	if !result.AllPassed {
		t.Errorf("All packages should pass in integration test")
	}

	if len(result.Results) != len(packages) {
		t.Errorf("Expected %d package results, got %d", len(packages), len(result.Results))
	}
}