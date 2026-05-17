package monitoringvalidator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validAlertRulesYAML = `groups:
  - name: mem0
    rules:
      - alert: Mem0APIDown
        expr: up{job="mem0-api"} == 0
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Mem0 API is down"
  - name: k3s
    rules:
      - alert: K3sNodeNotReady
        expr: kube_node_status_condition{condition="Ready",status="true"} == 0
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "K3s node not ready"
  - name: vllm
    rules:
      - alert: VLLMGPUOOMRisk
        expr: nvidia_gpu_memory_used_bytes / nvidia_gpu_memory_total_bytes > 0.95
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "GPU memory usage above 95%"
  - name: fleet-health
    rules:
      - alert: FleetNodeDown
        expr: fleet_node_up == 0
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Fleet node is down"
`

func TestValidateAlertRules_Valid(t *testing.T) {
	reqs := DefaultAlertRuleRequirements()
	result, err := ValidateAlertRules([]byte(validAlertRulesYAML), reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "valid alert rules should pass; failures: %v", result.Failures)
}

func TestValidateAlertRules_MissingGroup(t *testing.T) {
	yaml := `groups:
  - name: mem0
    rules:
      - alert: Mem0APIDown
        expr: up{job="mem0-api"} == 0
        for: 5m
        labels:
          severity: critical
  - name: k3s
    rules:
      - alert: K3sNodeNotReady
        expr: kube_node_status_condition{condition="Ready",status="true"} == 0
        for: 10m
        labels:
          severity: warning
`
	reqs := DefaultAlertRuleRequirements()
	result, err := ValidateAlertRules([]byte(yaml), reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "should fail with missing groups (vllm, fleet-health)")
}

func TestValidateAlertRules_MissingSeverityLabel(t *testing.T) {
	yaml := `groups:
  - name: mem0
    rules:
      - alert: Mem0APIDown
        expr: up{job="mem0-api"} == 0
        for: 5m
        annotations:
          summary: "Mem0 API is down"
  - name: k3s
    rules:
      - alert: K3sNodeNotReady
        expr: kube_node_status_condition{condition="Ready",status="true"} == 0
        for: 10m
        labels:
          severity: warning
  - name: vllm
    rules:
      - alert: VLLMGPUOOMRisk
        expr: nvidia_gpu_memory_used_bytes / nvidia_gpu_memory_total_bytes > 0.95
        for: 5m
        labels:
          severity: critical
  - name: fleet-health
    rules:
      - alert: FleetNodeDown
        expr: fleet_node_up == 0
        for: 5m
        labels:
          severity: critical
`
	reqs := DefaultAlertRuleRequirements()
	result, err := ValidateAlertRules([]byte(yaml), reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "should fail: Mem0APIDown missing severity label")
}

func TestValidateAlertRules_EmptyGroups(t *testing.T) {
	yaml := `groups: []`
	reqs := DefaultAlertRuleRequirements()
	result, err := ValidateAlertRules([]byte(yaml), reqs)
	require.NoError(t, err)
	assert.False(t, result.OK())
}

func TestValidateAlertRules_EmptyRulesInGroup(t *testing.T) {
	yaml := `groups:
  - name: mem0
    rules: []
  - name: k3s
    rules: []
  - name: vllm
    rules: []
  - name: fleet-health
    rules: []
`
	reqs := DefaultAlertRuleRequirements()
	result, err := ValidateAlertRules([]byte(yaml), reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "groups with no rules should fail")
}
