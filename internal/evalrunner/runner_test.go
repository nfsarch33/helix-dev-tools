package evalrunner_test

import (
	"context"
	"testing"
	"time"

	"github.com/nfsarch33/helix-dev-tools/internal/evalrunner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunner_ExecSuccess(t *testing.T) {
	r := evalrunner.NewRunner(evalrunner.Config{Timeout: 5 * time.Second})
	result := r.ExecCriterion(context.Background(), evalrunner.Criterion{
		Name:    "echo-test",
		Command: "echo hello",
		Check:   "exit_code_zero",
	})
	assert.True(t, result.Pass)
	assert.Contains(t, result.Output, "hello")
	assert.Less(t, result.Duration, 3*time.Second)
}

func TestRunner_ExecFailure(t *testing.T) {
	r := evalrunner.NewRunner(evalrunner.Config{Timeout: 5 * time.Second})
	result := r.ExecCriterion(context.Background(), evalrunner.Criterion{
		Name:    "fail-test",
		Command: "exit 1",
		Check:   "exit_code_zero",
	})
	assert.False(t, result.Pass)
}

func TestRunner_Timeout(t *testing.T) {
	r := evalrunner.NewRunner(evalrunner.Config{Timeout: 500 * time.Millisecond})
	result := r.ExecCriterion(context.Background(), evalrunner.Criterion{
		Name:    "slow-test",
		Command: "sleep 10",
		Check:   "exit_code_zero",
	})
	assert.False(t, result.Pass)
	assert.Contains(t, result.Error, "timeout")
}

func TestRunner_RunEval(t *testing.T) {
	r := evalrunner.NewRunner(evalrunner.Config{Timeout: 5 * time.Second})
	eval := evalrunner.EvalDef{
		Name: "basic-suite",
		Criteria: []evalrunner.Criterion{
			{Name: "c1", Command: "echo pass1", Check: "exit_code_zero"},
			{Name: "c2", Command: "echo pass2", Check: "exit_code_zero"},
		},
		PassThreshold: 1.0,
	}
	result := r.RunEval(context.Background(), eval)
	assert.True(t, result.Pass)
	assert.Equal(t, 1.0, result.PassRate)
	assert.Len(t, result.CriteriaResults, 2)
}

func TestRunner_RunEvalPartialFail(t *testing.T) {
	r := evalrunner.NewRunner(evalrunner.Config{Timeout: 5 * time.Second})
	eval := evalrunner.EvalDef{
		Name: "mixed-suite",
		Criteria: []evalrunner.Criterion{
			{Name: "c1", Command: "echo pass", Check: "exit_code_zero"},
			{Name: "c2", Command: "exit 1", Check: "exit_code_zero"},
		},
		PassThreshold: 1.0,
	}
	result := r.RunEval(context.Background(), eval)
	assert.False(t, result.Pass)
	assert.Equal(t, 0.5, result.PassRate)
}

func TestRunner_OutputContainsCheck(t *testing.T) {
	r := evalrunner.NewRunner(evalrunner.Config{Timeout: 5 * time.Second})
	result := r.ExecCriterion(context.Background(), evalrunner.Criterion{
		Name:    "output-check",
		Command: "echo PASS_MARKER_XYZ",
		Check:   "output_contains",
		Pattern: "PASS_MARKER_XYZ",
	})
	assert.True(t, result.Pass)
}

func TestRunner_OutputContainsFail(t *testing.T) {
	r := evalrunner.NewRunner(evalrunner.Config{Timeout: 5 * time.Second})
	result := r.ExecCriterion(context.Background(), evalrunner.Criterion{
		Name:    "output-check-fail",
		Command: "echo something_else",
		Check:   "output_contains",
		Pattern: "EXPECTED_NOT_FOUND",
	})
	assert.False(t, result.Pass)
}

func TestConfig_Defaults(t *testing.T) {
	cfg := evalrunner.DefaultConfig()
	assert.Equal(t, 30*time.Second, cfg.Timeout)
	assert.Equal(t, 3, cfg.MaxRetries)
}

func TestCriterionResult_Fields(t *testing.T) {
	r := evalrunner.CriterionResult{
		Name:     "test",
		Pass:     true,
		Duration: 100 * time.Millisecond,
		Output:   "hello",
	}
	require.Equal(t, "test", r.Name)
	assert.True(t, r.Pass)
}
