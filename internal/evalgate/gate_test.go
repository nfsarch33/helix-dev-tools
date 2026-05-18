package evalgate_test

import (
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/evalgate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvalDef_Parse(t *testing.T) {
	yaml := `
name: sprintboard-crud
type: capability
criteria:
  - name: ticket_create
    check: exit_code_zero
    command: "go test -run TestTicketCreate ./internal/platform/sprintboard/"
  - name: ticket_search
    check: exit_code_zero
    command: "go test -run TestTicketSearch ./internal/platform/sprintboard/"
pass_threshold: 1.0
`
	def, err := evalgate.ParseEvalDef([]byte(yaml))
	require.NoError(t, err)
	assert.Equal(t, "sprintboard-crud", def.Name)
	assert.Equal(t, "capability", def.Type)
	assert.Len(t, def.Criteria, 2)
	assert.Equal(t, 1.0, def.PassThreshold)
}

func TestEvalDef_Validate(t *testing.T) {
	def := evalgate.EvalDef{
		Name:          "test",
		Type:          "capability",
		Criteria:      []evalgate.Criterion{{Name: "c1", Check: "exit_code_zero", Command: "echo ok"}},
		PassThreshold: 1.0,
	}
	assert.NoError(t, def.Validate())
}

func TestEvalDef_ValidateEmpty(t *testing.T) {
	def := evalgate.EvalDef{}
	assert.Error(t, def.Validate())
}

func TestBatchConfig_Parse(t *testing.T) {
	yaml := `
evals:
  - path: evals/sprintboard.yaml
  - path: evals/mem0bridge.yaml
  - path: evals/agentrace.yaml
fail_fast: true
`
	cfg, err := evalgate.ParseBatchConfig([]byte(yaml))
	require.NoError(t, err)
	assert.Len(t, cfg.Evals, 3)
	assert.True(t, cfg.FailFast)
}

func TestBaseline_Compare(t *testing.T) {
	baseline := evalgate.Baseline{
		Name:      "sprintboard-crud",
		PassRate:  1.0,
		Timestamp: "2026-05-19T10:00:00+10:00",
	}
	current := evalgate.EvalResult{PassRate: 0.8}
	regression := baseline.DetectRegression(current, 0.05)
	assert.True(t, regression)
}

func TestBaseline_NoRegression(t *testing.T) {
	baseline := evalgate.Baseline{PassRate: 0.9}
	current := evalgate.EvalResult{PassRate: 0.95}
	regression := baseline.DetectRegression(current, 0.05)
	assert.False(t, regression)
}

func TestSprintGate_Pass(t *testing.T) {
	gate := evalgate.SprintGate{
		Baselines: []evalgate.Baseline{
			{Name: "crud", PassRate: 1.0},
		},
		Results: []evalgate.EvalResult{
			{Name: "crud", PassRate: 1.0},
		},
		RegressionThreshold: 0.05,
	}
	verdict := gate.Evaluate()
	assert.True(t, verdict.Pass)
	assert.Empty(t, verdict.Regressions)
}

func TestSprintGate_Fail(t *testing.T) {
	gate := evalgate.SprintGate{
		Baselines: []evalgate.Baseline{
			{Name: "crud", PassRate: 1.0},
			{Name: "search", PassRate: 0.9},
		},
		Results: []evalgate.EvalResult{
			{Name: "crud", PassRate: 0.7},
			{Name: "search", PassRate: 0.85},
		},
		RegressionThreshold: 0.05,
	}
	verdict := gate.Evaluate()
	assert.False(t, verdict.Pass)
	assert.Len(t, verdict.Regressions, 2)
}

func TestEvalResult_ToNDJSON(t *testing.T) {
	r := evalgate.EvalResult{Name: "test-eval", PassRate: 0.95, CriteriaResults: map[string]bool{"c1": true, "c2": true}}
	line := r.ToNDJSON()
	assert.Contains(t, line, "test-eval")
	assert.Contains(t, line, "0.95")
}
