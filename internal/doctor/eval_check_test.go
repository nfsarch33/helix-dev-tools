package doctor

import "testing"

func TestCheckSprintboardBinary(t *testing.T) {
	result := CheckSprintboardBinary()
	if result.Name != "sprintboard-binary" {
		t.Errorf("name = %q, want sprintboard-binary", result.Name)
	}
	if result.Status != "PASS" && result.Status != "FAIL" {
		t.Errorf("status = %q, want PASS or FAIL", result.Status)
	}
}

func TestCheckSprintboardDB(t *testing.T) {
	result := CheckSprintboardDB()
	if result.Name != "sprintboard-db" {
		t.Errorf("name = %q, want sprintboard-db", result.Name)
	}
}

func TestCheckEvalPackage(t *testing.T) {
	result := CheckEvalPackage()
	if result.Name != "eval-package" {
		t.Errorf("name = %q, want eval-package", result.Name)
	}
}

func TestCheckEvalSkills(t *testing.T) {
	result := CheckEvalSkills()
	if result.Name != "eval-skills" {
		t.Errorf("name = %q, want eval-skills", result.Name)
	}
}

func TestRunAllChecks(t *testing.T) {
	checks := RunAllChecks()
	if len(checks) != 4 {
		t.Errorf("got %d checks, want 4", len(checks))
	}
	for _, c := range checks {
		if c.Name == "" {
			t.Error("check name should not be empty")
		}
		if c.Status == "" {
			t.Errorf("check %q has empty status", c.Name)
		}
	}
}

func TestCheckResult_ValidStatuses(t *testing.T) {
	checks := RunAllChecks()
	valid := map[string]bool{"PASS": true, "FAIL": true, "WARN": true}
	for _, c := range checks {
		if !valid[c.Status] {
			t.Errorf("check %q has invalid status %q", c.Name, c.Status)
		}
	}
}
