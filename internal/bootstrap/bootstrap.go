package bootstrap

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type Step struct {
	Name    string `json:"name" yaml:"name"`
	Command string `json:"command" yaml:"command"`
	Check   string `json:"check,omitempty" yaml:"check,omitempty"`
	Skip    bool   `json:"skip,omitempty" yaml:"skip,omitempty"`
}

type Config struct {
	Steps []Step `json:"steps" yaml:"steps"`
}

type StepResult struct {
	Name       string        `json:"name"`
	Status     string        `json:"status"`
	DurationMS int64         `json:"duration_ms"`
	Error      string        `json:"error,omitempty"`
}

type BootstrapResult struct {
	TotalSteps int            `json:"total_steps"`
	Passed     int            `json:"passed"`
	Failed     int            `json:"failed"`
	Skipped    int            `json:"skipped"`
	Results    []StepResult   `json:"results"`
	DurationMS int64          `json:"duration_ms"`
}

func DefaultConfig() Config {
	home, _ := os.UserHomeDir()
	return Config{
		Steps: []Step{
			{Name: "go-version", Check: "go version"},
			{Name: "git-version", Check: "git --version"},
			{Name: "global-kb", Check: fmt.Sprintf("test -d %s", filepath.Join(home, "Code", "global-kb"))},
			{Name: "cursor-tools-binary", Check: fmt.Sprintf("test -x %s", filepath.Join(home, "bin", "cursor-tools"))},
			{Name: "runx-binary", Check: fmt.Sprintf("test -x %s", filepath.Join(home, "runs", "runx"))},
			{Name: "mem0-mcp-binary", Check: fmt.Sprintf("test -x %s", filepath.Join(home, "runs", "mem0-mcp-go"))},
			{Name: "sprintboard-mcp-binary", Check: fmt.Sprintf("test -x %s", filepath.Join(home, "runs", "sprintboard-mcp"))},
			{Name: "cursor-rules", Check: fmt.Sprintf("test -d %s", filepath.Join(home, ".cursor", "rules"))},
			{Name: "cursor-hooks", Check: fmt.Sprintf("test -f %s", filepath.Join(home, ".cursor", "hooks.json"))},
			{Name: "cursor-mcp", Check: fmt.Sprintf("test -f %s", filepath.Join(home, ".cursor", "mcp.json"))},
			{Name: "runx-config", Check: fmt.Sprintf("test -f %s", filepath.Join(home, ".config", "runx", "config.yaml"))},
			{Name: "owners-manifest", Check: fmt.Sprintf("test -f %s", filepath.Join(home, ".config", "runx", "owners.yaml"))},
		},
	}
}

func Verify(cfg Config) BootstrapResult {
	start := time.Now()
	result := BootstrapResult{TotalSteps: len(cfg.Steps)}

	for _, step := range cfg.Steps {
		if step.Skip {
			result.Skipped++
			result.Results = append(result.Results, StepResult{
				Name: step.Name, Status: "skipped",
			})
			continue
		}

		stepStart := time.Now()
		checkCmd := step.Check
		if checkCmd == "" {
			checkCmd = step.Command
		}

		sr := StepResult{Name: step.Name}

		if checkCmd == "" {
			sr.Status = "skipped"
			result.Skipped++
		} else {
			cmd := exec.Command("sh", "-c", checkCmd)
			if err := cmd.Run(); err != nil {
				sr.Status = "failed"
				sr.Error = err.Error()
				result.Failed++
			} else {
				sr.Status = "passed"
				result.Passed++
			}
		}

		sr.DurationMS = time.Since(stepStart).Milliseconds()
		result.Results = append(result.Results, sr)
	}

	result.DurationMS = time.Since(start).Milliseconds()
	return result
}

func (r *BootstrapResult) AllPassed() bool {
	return r.Failed == 0
}

func (r *BootstrapResult) Summary() string {
	status := "PASS"
	if !r.AllPassed() {
		status = "FAIL"
	}
	return fmt.Sprintf("Bootstrap %s: %d/%d passed, %d failed, %d skipped (%.1fs)",
		status, r.Passed, r.TotalSteps, r.Failed, r.Skipped,
		float64(r.DurationMS)/1000.0)
}
