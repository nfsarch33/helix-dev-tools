package batchvalidate

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Runner is a test runner that executes go test -race for a batch.
type Runner struct {
	GoCmd   string
	Timeout time.Duration
	Env     []string
}

// PackageResult holds the result of one go test run.
type PackageResult struct {
	Package   string
	Passed    bool
	Output    string
	Duration time.Duration
	Error    error
}

// BatchResult holds results for all packages in a batch.
type BatchResult struct {
	WorktreePath string
	Branch       string
	Results      []PackageResult
	AllPassed    bool
	Duration    time.Duration
}

// RunnerOption configures Runner behavior.
type RunnerOption func(*Runner)

// WithGoCmd sets the go command path
func WithGoCmd(cmd string) RunnerOption {
	return func(r *Runner) {
		r.GoCmd = cmd
	}
}

// WithTimeout sets the per-package test timeout
func WithTimeout(d time.Duration) RunnerOption {
	return func(r *Runner) {
		r.Timeout = d
	}
}

// New returns a Runner with defaults.
func New(opts ...RunnerOption) *Runner {
	r := &Runner{
		GoCmd:   "go",
		Timeout: 60 * time.Second,
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// ValidateBatch runs go test -race -count=1 on all ./internal/... packages
func (r *Runner) ValidateBatch(ctx context.Context, worktreePath string) (BatchResult, error) {
	startTime := time.Now()
	batchResult := BatchResult{
		WorktreePath: worktreePath,
		AllPassed:    true,
	}

	// Find internal packages
	packages, err := r.findInternalPackages(worktreePath)
	if err != nil {
		return batchResult, err
	}

	for _, pkg := range packages {
		result := r.runPackageTest(ctx, worktreePath, pkg)
		batchResult.Results = append(batchResult.Results, result)

		if !result.Passed {
			batchResult.AllPassed = false
		}
	}

	batchResult.Duration = time.Since(startTime)
	return batchResult, nil
}

// findInternalPackages discovers all packages under ./internal/...
func (r *Runner) findInternalPackages(worktreePath string) ([]string, error) {
	cmd := exec.Command(r.GoCmd, "list", "./internal/...")
	cmd.Dir = worktreePath
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("could not list packages: %v", err)
	}

	packages := strings.Split(strings.TrimSpace(string(output)), "\n")
	return packages, nil
}

// runPackageTest executes go test -race for a single package
func (r *Runner) runPackageTest(ctx context.Context, worktreePath, pkg string) PackageResult {
	start := time.Now()
	result := PackageResult{
		Package: pkg,
		Passed:  false,
	}

	// Construct command
	cmd := exec.CommandContext(ctx, r.GoCmd, "test", "-race", "-count=1", pkg)
	cmd.Dir = worktreePath
	cmd.Env = append(os.Environ(), r.Env...)

	// Set timeout
	timer := time.AfterFunc(r.Timeout, func() {
		cmd.Process.Kill()
	})
	defer timer.Stop()

	// Run test
	output, err := cmd.CombinedOutput()
	result.Duration = time.Since(start)
	result.Output = string(output)

	if err == nil {
		result.Passed = true
	} else {
		result.Error = err
	}

	return result
}

// ValidateBatchDry creates a stub BatchResult
func (r *Runner) ValidateBatchDry(worktreePath string, packageNames []string) BatchResult {
	result := BatchResult{
		WorktreePath: worktreePath,
		AllPassed:    true,
	}

	for _, pkg := range packageNames {
		result.Results = append(result.Results, PackageResult{
			Package:   pkg,
			Passed:    true,
			Duration:  10 * time.Millisecond,
		})
	}

	return result
}

// FormatReport generates a human-readable summary
func FormatReport(result BatchResult) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Batch Validation for %s:\n", result.WorktreePath))
	sb.WriteString(fmt.Sprintf("Total Duration: %v\n", result.Duration))
	sb.WriteString(fmt.Sprintf("Overall Status: %v\n", result.AllPassed))
	sb.WriteString("Package Results:\n")

	for _, pkgResult := range result.Results {
		status := "PASS"
		if !pkgResult.Passed {
			status = "FAIL"
		}
		sb.WriteString(fmt.Sprintf("  %s [%s] (%v)\n", pkgResult.Package, status, pkgResult.Duration))

		if !pkgResult.Passed {
			sb.WriteString(fmt.Sprintf("    Error: %v\n", pkgResult.Error))
			sb.WriteString(fmt.Sprintf("    Output:\n%s\n", pkgResult.Output))
		}
	}

	return sb.String()
}