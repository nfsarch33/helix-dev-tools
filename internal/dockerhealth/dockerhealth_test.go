package dockerhealth_test

import (
	"os"
	"testing"

	"github.com/nfsarch33/cursor-tools/internal/dockerhealth"
)

func TestParseDockerPS_ValidOutput(t *testing.T) {
	output := `CONTAINER ID|mem0-api|mem0/mem0:latest|Up 2 hours (healthy)|running|always|0
CONTAINER ID|mem0-postgres|postgres:15|Up 2 hours (healthy)|running|always|0
CONTAINER ID|mem0-qdrant|qdrant/qdrant:latest|Up 2 hours|running|unless-stopped|0
CONTAINER ID|mem0-redis|redis:7-alpine|Up 2 hours|running|always|0`

	result := dockerhealth.ParseDockerPS(output, "test-host")
	if result.Total != 4 {
		t.Fatalf("expected 4 containers, got %d", result.Total)
	}
	if result.Healthy != 4 {
		t.Errorf("expected 4 healthy, got %d", result.Healthy)
	}
}

func TestParseDockerPS_UnhealthyContainer(t *testing.T) {
	output := `CONTAINER ID|mem0-api|mem0/mem0:latest|Up 2 hours (unhealthy)|running|always|3
CONTAINER ID|mem0-postgres|postgres:15|Up 2 hours (healthy)|running|always|0`

	result := dockerhealth.ParseDockerPS(output, "test-host")
	if result.Unhealthy != 1 {
		t.Errorf("expected 1 unhealthy, got %d", result.Unhealthy)
	}
	if result.Containers[0].RestartCount != 3 {
		t.Errorf("expected restart count 3, got %d", result.Containers[0].RestartCount)
	}
}

func TestParseDockerPS_EmptyOutput(t *testing.T) {
	result := dockerhealth.ParseDockerPS("", "test-host")
	if result.Total != 0 {
		t.Fatalf("expected 0 containers for empty output, got %d", result.Total)
	}
}

func TestValidateRestartPolicies(t *testing.T) {
	containers := []dockerhealth.ContainerState{
		{Name: "api", RestartPolicy: "always"},
		{Name: "db", RestartPolicy: "unless-stopped"},
		{Name: "cache", RestartPolicy: "no"},
	}

	errors := dockerhealth.ValidateRestartPolicies(containers, "always")
	if len(errors) != 2 {
		t.Fatalf("expected 2 errors (db and cache), got %d: %v", len(errors), errors)
	}
}

func TestValidateRestartPolicies_AllCompliant(t *testing.T) {
	containers := []dockerhealth.ContainerState{
		{Name: "api", RestartPolicy: "always"},
		{Name: "db", RestartPolicy: "always"},
	}

	errors := dockerhealth.ValidateRestartPolicies(containers, "always")
	if len(errors) != 0 {
		t.Fatalf("expected 0 errors, got %d: %v", len(errors), errors)
	}
}

func TestParseAlertRuleFile_ValidYAML(t *testing.T) {
	yaml := `---
groups:
  - name: mem0
    rules:
      - alert: Mem0APIDown
        expr: mem0_api_up == 0
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "Mem0 API is down"
`
	result, err := dockerhealth.ParseAlertRuleFile([]byte(yaml))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(result.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(result.Groups))
	}
	if len(result.Groups[0].Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(result.Groups[0].Rules))
	}
	if result.Groups[0].Rules[0].Alert != "Mem0APIDown" {
		t.Errorf("expected alert name Mem0APIDown, got %s", result.Groups[0].Rules[0].Alert)
	}
}

func TestParseAlertRuleFile_InvalidYAML(t *testing.T) {
	_, err := dockerhealth.ParseAlertRuleFile([]byte("not valid yaml: {["))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestValidateAlertRule_MissingExpr(t *testing.T) {
	rule := dockerhealth.AlertRule{
		Alert: "TestAlert",
		For:   "2m",
	}
	errs := dockerhealth.ValidateAlertRule(rule)
	if len(errs) == 0 {
		t.Fatal("expected error for missing expr")
	}
}

func TestValidateAlertRule_Valid(t *testing.T) {
	rule := dockerhealth.AlertRule{
		Alert: "TestAlert",
		Expr:  "up == 0",
		For:   "2m",
		Labels: map[string]string{
			"severity": "critical",
		},
	}
	errs := dockerhealth.ValidateAlertRule(rule)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
}

func TestIntegration_DockerHealthCheck(t *testing.T) {
	if os.Getenv("CURSOR_TOOLS_INTEGRATION") != "1" {
		t.Skip("set CURSOR_TOOLS_INTEGRATION=1 to run Docker health integration tests")
	}

	alias := os.Getenv("CURSOR_TOOLS_DOCKER_HOST_ALIAS")
	if alias == "" {
		t.Skip("set CURSOR_TOOLS_DOCKER_HOST_ALIAS for Docker health check")
	}

	result := dockerhealth.RemoteDockerHealthCheck(alias)
	t.Logf("Docker health: %d/%d healthy, errors: %v",
		result.Healthy, result.Total, result.Errors)

	if result.Total == 0 && len(result.Errors) == 0 {
		t.Log("no containers found (host may be clean)")
	}
}
