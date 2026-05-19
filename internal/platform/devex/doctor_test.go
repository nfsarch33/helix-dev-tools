package devex

import (
	"strings"
	"testing"
)

func TestDiagnosticRunner_Run(t *testing.T) {
	d := NewDiagnosticRunner()
	d.Register(func() DiagnosticCheck {
		return DiagnosticCheck{Name: "binaries", Status: CheckPass, Message: "all present"}
	})
	d.Register(func() DiagnosticCheck {
		return DiagnosticCheck{Name: "tunnel", Status: CheckFail, Message: "not connected", FixHint: "runx tunnel start"}
	})

	results := d.Run()
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestSummarizeResults(t *testing.T) {
	results := []DiagnosticCheck{
		{Status: CheckPass},
		{Status: CheckPass},
		{Status: CheckWarn},
		{Status: CheckFail},
	}
	pass, warn, fail := SummarizeResults(results)
	if pass != 2 || warn != 1 || fail != 1 {
		t.Errorf("expected 2/1/1, got %d/%d/%d", pass, warn, fail)
	}
}

func TestOverallStatus_Green(t *testing.T) {
	results := []DiagnosticCheck{{Status: CheckPass}, {Status: CheckPass}}
	if OverallStatus(results) != "GREEN" {
		t.Error("expected GREEN")
	}
}

func TestOverallStatus_Yellow(t *testing.T) {
	results := []DiagnosticCheck{{Status: CheckPass}, {Status: CheckWarn}}
	if OverallStatus(results) != "YELLOW" {
		t.Error("expected YELLOW")
	}
}

func TestOverallStatus_Red(t *testing.T) {
	results := []DiagnosticCheck{{Status: CheckFail}}
	if OverallStatus(results) != "RED" {
		t.Error("expected RED")
	}
}

func TestFormatResults(t *testing.T) {
	results := []DiagnosticCheck{
		{Name: "check1", Status: CheckPass, Message: "ok"},
		{Name: "check2", Status: CheckFail, Message: "broken", FixHint: "fix it"},
	}
	output := FormatResults(results)
	if !strings.Contains(output, "[PASS]") {
		t.Error("missing PASS")
	}
	if !strings.Contains(output, "[FAIL]") {
		t.Error("missing FAIL")
	}
	if !strings.Contains(output, "Fix: fix it") {
		t.Error("missing fix hint")
	}
}

func TestDiagnosticRunner_CheckCount(t *testing.T) {
	d := NewDiagnosticRunner()
	d.Register(func() DiagnosticCheck { return DiagnosticCheck{} })
	d.Register(func() DiagnosticCheck { return DiagnosticCheck{} })

	if d.CheckCount() != 2 {
		t.Errorf("expected 2, got %d", d.CheckCount())
	}
}
