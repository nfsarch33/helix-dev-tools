package doctor

import (
	"fmt"
	"os"
	"path/filepath"
)

type CheckResult struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

func CheckEvalPackage() CheckResult {
	home, _ := os.UserHomeDir()
	evalDir := filepath.Join(home, "cursor-tools", "internal", "eval")

	requiredFiles := []string{"runner.go", "types.go", "grader.go", "metrics.go", "report.go", "loader.go", "quality.go"}

	missing := 0
	for _, f := range requiredFiles {
		if _, err := os.Stat(filepath.Join(evalDir, f)); os.IsNotExist(err) {
			missing++
		}
	}

	if missing > 0 {
		return CheckResult{
			Name:   "eval-package",
			Status: "FAIL",
			Detail: fmt.Sprintf("%d/%d eval files missing", missing, len(requiredFiles)),
		}
	}

	return CheckResult{
		Name:   "eval-package",
		Status: "PASS",
		Detail: fmt.Sprintf("%d eval files present", len(requiredFiles)),
	}
}

func CheckSprintboardDB() CheckResult {
	home, _ := os.UserHomeDir()
	dbPath := filepath.Join(home, ".config", "helix-dev-tools", "sprintboard.db")

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return CheckResult{
			Name:   "sprintboard-db",
			Status: "WARN",
			Detail: "sprintboard.db not found (created on first MCP call)",
		}
	}

	info, err := os.Stat(dbPath)
	if err != nil {
		return CheckResult{
			Name:   "sprintboard-db",
			Status: "FAIL",
			Detail: err.Error(),
		}
	}

	return CheckResult{
		Name:   "sprintboard-db",
		Status: "PASS",
		Detail: fmt.Sprintf("%.1f KB", float64(info.Size())/1024.0),
	}
}

func CheckSprintboardBinary() CheckResult {
	home, _ := os.UserHomeDir()
	binPath := filepath.Join(home, "runs", "sprintboard-mcp")

	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		return CheckResult{
			Name:   "sprintboard-binary",
			Status: "FAIL",
			Detail: "~/runs/sprintboard-mcp not found",
		}
	}

	return CheckResult{
		Name:   "sprintboard-binary",
		Status: "PASS",
		Detail: binPath,
	}
}

func CheckEvalSkills() CheckResult {
	home, _ := os.UserHomeDir()
	skills := []string{"eval-harness", "evaluation-methodology", "iterative-retrieval"}

	found := 0
	for _, s := range skills {
		path := filepath.Join(home, ".cursor", "skills", s, "SKILL.md")
		if _, err := os.Stat(path); err == nil {
			found++
		}
	}

	if found < len(skills) {
		return CheckResult{
			Name:   "eval-skills",
			Status: "WARN",
			Detail: fmt.Sprintf("%d/%d eval skills installed", found, len(skills)),
		}
	}

	return CheckResult{
		Name:   "eval-skills",
		Status: "PASS",
		Detail: fmt.Sprintf("%d eval skills installed", found),
	}
}

func RunAllChecks() []CheckResult {
	return []CheckResult{
		CheckEvalPackage(),
		CheckSprintboardDB(),
		CheckSprintboardBinary(),
		CheckEvalSkills(),
	}
}
