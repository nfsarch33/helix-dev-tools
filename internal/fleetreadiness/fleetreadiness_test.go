package fleetreadiness

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEvaluateReadinessPassesWhenAllChecksPass(t *testing.T) {
	result := EvaluateReadiness([]Check{
		{Name: "k3s", Passed: true, Required: true},
		{Name: "mem0", Passed: true, Required: true},
		{Name: "dashboard", Passed: true, Required: false},
	})

	assert.True(t, result.Ready)
	assert.Equal(t, 0, result.RequiredFailed)
}

func TestEvaluateReadinessFailsWhenRequiredCheckFails(t *testing.T) {
	result := EvaluateReadiness([]Check{
		{Name: "k3s", Passed: true, Required: true},
		{Name: "vllm", Passed: false, Required: true},
	})

	assert.False(t, result.Ready)
	assert.Equal(t, 1, result.RequiredFailed)
}

func TestEvaluateReadinessWarnsWhenOptionalCheckFails(t *testing.T) {
	result := EvaluateReadiness([]Check{
		{Name: "k3s", Passed: true, Required: true},
		{Name: "dashboard", Passed: false, Required: false},
	})

	assert.True(t, result.Ready)
	assert.Equal(t, 1, result.OptionalFailed)
}

func TestEvaluateReadinessRejectsEmptyCheckList(t *testing.T) {
	result := EvaluateReadiness(nil)

	assert.False(t, result.Ready)
	assert.NotEmpty(t, result.Errors)
}

func TestEvaluateReadinessCountsTotals(t *testing.T) {
	result := EvaluateReadiness([]Check{
		{Name: "a", Passed: true, Required: true},
		{Name: "b", Passed: false, Required: true},
		{Name: "c", Passed: false, Required: false},
	})

	assert.Equal(t, 3, result.Total)
	assert.Equal(t, 1, result.Passed)
	assert.Equal(t, 2, result.Failed)
}
