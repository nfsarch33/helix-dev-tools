package freshinstall

import (
	"testing"
)

func TestNewValidator(t *testing.T) {
	v := New(Config{HomePath: "/tmp/test-home"})
	if v == nil {
		t.Fatal("expected non-nil validator")
	}
}

func TestCheckComponent(t *testing.T) {
	v := New(Config{HomePath: "/tmp/test-home"})
	v.AddComponent(Component{Name: "runx", BinaryPath: "/usr/local/bin/runx"})
	v.AddComponent(Component{Name: "sprintboard-mcp", BinaryPath: "/tmp/runs/sprintboard-mcp"})

	components := v.Components()
	if len(components) != 2 {
		t.Fatalf("got %d components, want 2", len(components))
	}
}

func TestValidateResult(t *testing.T) {
	v := New(Config{HomePath: "/tmp/test-home"})
	v.RecordCheck("runx", CheckResult{Exists: true, Version: "1.2.3", Healthy: true})
	v.RecordCheck("mem0-mcp-go", CheckResult{Exists: true, Version: "0.5.0", Healthy: true})
	v.RecordCheck("sprintboard-mcp", CheckResult{Exists: false})

	results := v.Results()
	if results["runx"].Healthy != true {
		t.Error("expected runx healthy")
	}
	if results["sprintboard-mcp"].Exists {
		t.Error("expected sprintboard-mcp not exists")
	}
}

func TestAllHealthy(t *testing.T) {
	v := New(Config{HomePath: "/tmp/test-home"})
	v.RecordCheck("runx", CheckResult{Exists: true, Healthy: true})
	v.RecordCheck("mcp", CheckResult{Exists: true, Healthy: true})

	if !v.AllHealthy() {
		t.Error("expected all healthy")
	}
}

func TestNotAllHealthy(t *testing.T) {
	v := New(Config{HomePath: "/tmp/test-home"})
	v.RecordCheck("runx", CheckResult{Exists: true, Healthy: true})
	v.RecordCheck("tunnel", CheckResult{Exists: true, Healthy: false})

	if v.AllHealthy() {
		t.Error("expected not all healthy")
	}
}

func TestGenerateChecklist(t *testing.T) {
	v := New(Config{HomePath: "/tmp/test-home"})
	v.AddComponent(Component{Name: "runx", BinaryPath: "/usr/local/bin/runx", Required: true})
	v.AddComponent(Component{Name: "optional", BinaryPath: "/opt/bin/x", Required: false})

	checklist := v.GenerateChecklist()
	if len(checklist) != 2 {
		t.Fatalf("got %d checklist items, want 2", len(checklist))
	}
	if !checklist[0].Required {
		t.Error("expected first item required")
	}
}
