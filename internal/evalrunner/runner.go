package evalrunner

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// Criterion defines a single pass/fail check.
type Criterion struct {
	Name    string
	Command string
	Check   string
	Pattern string
}

// EvalDef is a suite of criteria to evaluate.
type EvalDef struct {
	Name          string
	Criteria      []Criterion
	PassThreshold float64
}

// CriterionResult captures one criterion's outcome.
type CriterionResult struct {
	Name     string
	Pass     bool
	Duration time.Duration
	Output   string
	Error    string
}

// EvalResult is the aggregate result of running an EvalDef.
type EvalResult struct {
	Name            string
	Pass            bool
	PassRate        float64
	CriteriaResults []CriterionResult
}

// Config controls runner behavior.
type Config struct {
	Timeout    time.Duration
	MaxRetries int
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{Timeout: 30 * time.Second, MaxRetries: 3}
}

// Runner executes eval criteria as shell commands.
type Runner struct {
	config Config
}

// NewRunner creates a runner with the given config.
func NewRunner(cfg Config) *Runner {
	return &Runner{config: cfg}
}

// ExecCriterion runs a single criterion and returns its result.
func (r *Runner) ExecCriterion(ctx context.Context, c Criterion) CriterionResult {
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, r.config.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", c.Command)
	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

	result := CriterionResult{
		Name:     c.Name,
		Duration: duration,
		Output:   string(output),
	}

	if ctx.Err() == context.DeadlineExceeded {
		result.Pass = false
		result.Error = "timeout: command exceeded " + r.config.Timeout.String()
		return result
	}

	switch c.Check {
	case "exit_code_zero":
		result.Pass = err == nil
		if err != nil {
			result.Error = err.Error()
		}
	case "output_contains":
		result.Pass = err == nil && strings.Contains(string(output), c.Pattern)
		if err != nil {
			result.Error = err.Error()
		}
	default:
		result.Pass = err == nil
	}

	return result
}

// RunEval executes all criteria in an EvalDef and returns the aggregate result.
func (r *Runner) RunEval(ctx context.Context, eval EvalDef) EvalResult {
	var results []CriterionResult
	passCount := 0

	for _, c := range eval.Criteria {
		cr := r.ExecCriterion(ctx, c)
		results = append(results, cr)
		if cr.Pass {
			passCount++
		}
	}

	passRate := 0.0
	if len(results) > 0 {
		passRate = float64(passCount) / float64(len(results))
	}

	return EvalResult{
		Name:            eval.Name,
		Pass:            passRate >= eval.PassThreshold,
		PassRate:        passRate,
		CriteriaResults: results,
	}
}
