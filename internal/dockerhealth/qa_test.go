package dockerhealth_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/nfsarch33/cursor-tools/internal/dockerhealth"
)

func TestQA_Mem0AlertRulesFile(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path")
	}
	testdata := filepath.Join(filepath.Dir(thisFile), "testdata", "mem0-alerts.yml")

	data, err := os.ReadFile(testdata)
	if err != nil {
		t.Fatalf("cannot read alert rules: %v", err)
	}

	ruleFile, err := dockerhealth.ParseAlertRuleFile(data)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(ruleFile.Groups) == 0 {
		t.Fatal("expected at least one alert group")
	}

	group := ruleFile.Groups[0]
	if group.Name != "mem0-selfhost" {
		t.Errorf("expected group name mem0-selfhost, got %s", group.Name)
	}

	if len(group.Rules) < 3 {
		t.Fatalf("expected at least 3 rules, got %d", len(group.Rules))
	}

	foundAPIDown := false
	for _, rule := range group.Rules {
		errs := dockerhealth.ValidateAlertRule(rule)
		if len(errs) > 0 {
			t.Errorf("rule %q has validation errors: %v", rule.Alert, errs)
		}
		if rule.Alert == "Mem0APIDown" {
			foundAPIDown = true
			if rule.For != "2m" {
				t.Errorf("Mem0APIDown should fire after 2m, got %s", rule.For)
			}
		}
	}

	if !foundAPIDown {
		t.Error("expected Mem0APIDown alert rule in the file")
	}
}

func TestQA_AlertRuleValidation_AllRequired(t *testing.T) {
	rules := []dockerhealth.AlertRule{
		{Alert: "", Expr: "up==0", For: "2m"},
		{Alert: "Test", Expr: "", For: "2m"},
		{Alert: "Test", Expr: "up==0", For: ""},
	}

	for i, rule := range rules {
		errs := dockerhealth.ValidateAlertRule(rule)
		if len(errs) == 0 {
			t.Errorf("rule[%d] should have validation errors", i)
		}
	}
}

func TestQA_DockerPSEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		total   int
		healthy int
	}{
		{
			name:    "single healthy container",
			output:  "ID|app|image:latest|Up 1h (healthy)|running|always|0",
			total:   1,
			healthy: 1,
		},
		{
			name:    "exited container",
			output:  "ID|app|image:latest|Exited (0) 1h ago|exited|always|0",
			total:   1,
			healthy: 0,
		},
		{
			name:    "high restart count",
			output:  "ID|app|image:latest|Up 5m (unhealthy)|running|always|15",
			total:   1,
			healthy: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := dockerhealth.ParseDockerPS(tc.output, "test")
			if result.Total != tc.total {
				t.Errorf("total: expected %d, got %d", tc.total, result.Total)
			}
			if result.Healthy != tc.healthy {
				t.Errorf("healthy: expected %d, got %d", tc.healthy, result.Healthy)
			}
		})
	}
}
