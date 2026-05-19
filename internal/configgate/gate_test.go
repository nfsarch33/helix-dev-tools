package configgate

import (
	"testing"
)

func TestNewGate(t *testing.T) {
	g := New(Config{ConfigPath: "~/.config/helix-dev-tools/config.yaml"})
	if g == nil {
		t.Fatal("expected non-nil gate")
	}
}

func TestValidateRule(t *testing.T) {
	g := New(Config{})
	g.AddRule(Rule{Key: "mem0.timeout", MinValue: "30s", MaxValue: "120s"})
	g.AddRule(Rule{Key: "sprintboard.max_agents", MinValue: "1", MaxValue: "10"})
	rules := g.Rules()
	if len(rules) != 2 {
		t.Fatalf("got %d rules, want 2", len(rules))
	}
}

func TestCheckPasses(t *testing.T) {
	g := New(Config{})
	g.AddRule(Rule{Key: "port", MinValue: "1024", MaxValue: "65535", Required: true})
	findings := g.Check(map[string]string{"port": "8080"})
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestCheckMissing(t *testing.T) {
	g := New(Config{})
	g.AddRule(Rule{Key: "api_key", Required: true})
	findings := g.Check(map[string]string{})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Kind != FindingMissing {
		t.Errorf("got kind %v, want missing", findings[0].Kind)
	}
}

func TestCheckOutOfRange(t *testing.T) {
	g := New(Config{})
	g.AddRule(Rule{Key: "timeout", MinValue: "10", MaxValue: "100"})
	findings := g.Check(map[string]string{"timeout": "200"})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Kind != FindingOutOfRange {
		t.Errorf("got kind %v, want out_of_range", findings[0].Kind)
	}
}
