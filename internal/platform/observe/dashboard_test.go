package observe

import (
	"strings"
	"testing"
)

func TestDashboard_Render(t *testing.T) {
	d := NewDashboard("System Health")
	d.AddPanel(DashboardPanel{Title: "Memory", Metric: "mem_free", Value: 69, Unit: "%", Status: "OK"})
	d.AddPanel(DashboardPanel{Title: "Latency", Metric: "p95_ms", Value: 45, Unit: "ms", Status: "WARN"})

	output := d.Render()
	if !strings.Contains(output, "System Health") {
		t.Error("missing title")
	}
	if !strings.Contains(output, "Memory") {
		t.Error("missing memory panel")
	}
}

func TestDashboard_PanelCount(t *testing.T) {
	d := NewDashboard("Test")
	d.AddPanel(DashboardPanel{Title: "A"})
	d.AddPanel(DashboardPanel{Title: "B"})

	if d.PanelCount() != 2 {
		t.Errorf("expected 2, got %d", d.PanelCount())
	}
}

func TestDashboard_StatusSummary(t *testing.T) {
	d := NewDashboard("Test")
	d.AddPanel(DashboardPanel{Status: "OK"})
	d.AddPanel(DashboardPanel{Status: "OK"})
	d.AddPanel(DashboardPanel{Status: "CRIT"})

	summary := d.StatusSummary()
	if summary["OK"] != 2 {
		t.Errorf("expected 2 OK, got %d", summary["OK"])
	}
	if summary["CRIT"] != 1 {
		t.Errorf("expected 1 CRIT, got %d", summary["CRIT"])
	}
}

func TestDashboard_DefaultStatus(t *testing.T) {
	d := NewDashboard("Test")
	d.AddPanel(DashboardPanel{Title: "NoStatus"})

	summary := d.StatusSummary()
	if summary["OK"] != 1 {
		t.Error("empty status should default to OK")
	}
}
