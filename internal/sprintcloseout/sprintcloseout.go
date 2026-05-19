// Package sprintcloseout provides evidence-gate checking for sprint closeout
// ceremonies, the 7-artefact contract required by sprint-scaffold-7-stories rule,
// and rebuild spec validation for the post-merge binary rebuild hook.
package sprintcloseout

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RequiredFiles returns the list of relative file paths that must exist in the
// global-kb sprint evidence directory for a given sprint ID to be considered
// complete. The 7 artefacts match the sprint-scaffold-7-stories rule.
func RequiredFiles(sprintID string) []string {
	return []string{
		filepath.Join("sprint-retros", sprintID+"-retro.md"),
		filepath.Join("reports", sprintID+"-kpi.md"),
		filepath.Join("global-memories", "capsules", sprintID+"-capsule.md"),
		filepath.Join("session-handoffs", sprintID+"-handoff.md"),
		filepath.Join("reports", "evidence", sprintID+"-evidence.md"),
		filepath.Join("reports", sprintID+"-badge.md"),
		filepath.Join("global-memories", "capsules", sprintID+"-evospine.md"),
	}
}

// CheckResult holds the outcome of a closeout evidence check.
type CheckResult struct {
	SprintID string
	Missing  []string
	OK       bool
}

// Report returns a human-readable one-line summary.
func (r CheckResult) Report() string {
	if r.OK {
		return fmt.Sprintf("[%s] COMPLETE -- all 7 evidence artefacts present", r.SprintID)
	}
	return fmt.Sprintf("[%s] INCOMPLETE -- missing %d/%d: %s",
		r.SprintID, len(r.Missing), 7, strings.Join(r.Missing, ", "))
}

// Check verifies the 7 required closeout evidence files under baseDir for the
// given sprint ID. baseDir should be the root of the global-kb checkout.
func Check(sprintID, baseDir string) CheckResult {
	result := CheckResult{SprintID: sprintID}
	for _, rel := range RequiredFiles(sprintID) {
		full := filepath.Join(baseDir, rel)
		if _, err := os.Stat(full); err != nil {
			result.Missing = append(result.Missing, rel)
		}
	}
	result.OK = len(result.Missing) == 0
	return result
}

// RebuildSpec describes a post-merge binary rebuild step. It names the repo
// and the Makefile target to invoke after a merge lands on main.
type RebuildSpec struct {
	// RepoPath is the absolute path to the git repository.
	RepoPath string
	// MakeTarget is the Makefile target to run (e.g., "install", "build").
	MakeTarget string
}

// Validate returns an error if the spec is missing required fields.
func (s RebuildSpec) Validate() error {
	var errs []string
	if s.RepoPath == "" {
		errs = append(errs, "RepoPath is required")
	}
	if s.MakeTarget == "" {
		errs = append(errs, "MakeTarget is required")
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

// BuildCommand returns the shell command string for the rebuild step.
func (s RebuildSpec) BuildCommand() string {
	return fmt.Sprintf("make -C %s %s", s.RepoPath, s.MakeTarget)
}
