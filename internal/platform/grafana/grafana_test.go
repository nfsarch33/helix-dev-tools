package grafana

import (
	"encoding/json"
	"testing"
)

func TestAddPanel_Lookup(t *testing.T) {
	d := NewDashboard("Fleet Overview", "fleet-overview")
	d.AddPanel(Panel{Title: "Active Agents", Type: PanelStat, Query: `count(agent_up)`})
	if len(d.Panels) != 1 {
		t.Fatalf("expected 1 panel, got %d", len(d.Panels))
	}
}

func TestAddPanel_AutoID(t *testing.T) {
	d := NewDashboard("Test", "test-uid")
	d.AddPanel(Panel{Title: "A", Query: "up"})
	d.AddPanel(Panel{Title: "B", Query: "down"})
	if d.Panels[0].ID != 1 || d.Panels[1].ID != 2 {
		t.Errorf("expected auto IDs 1 and 2, got %d and %d", d.Panels[0].ID, d.Panels[1].ID)
	}
}

func TestValidate_Valid(t *testing.T) {
	d := NewDashboard("OK", "ok-uid")
	d.AddPanel(Panel{Title: "P1", Query: "some_metric"})
	if errs := d.Validate(); len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidate_MissingTitle(t *testing.T) {
	d := NewDashboard("Bad", "bad-uid")
	d.AddPanel(Panel{Query: "metric"})
	if errs := d.Validate(); len(errs) == 0 {
		t.Error("expected error for missing panel title")
	}
}

func TestValidate_MissingQuery(t *testing.T) {
	d := NewDashboard("Bad", "bad-uid")
	d.AddPanel(Panel{Title: "NoQuery"})
	if errs := d.Validate(); len(errs) == 0 {
		t.Error("expected error for missing panel query")
	}
}

func TestToJSON_RoundTrip(t *testing.T) {
	d := NewDashboard("Sprint Velocity", "sprint-velocity")
	d.AddPanel(Panel{Title: "Pairs/Day", Type: PanelTimeSeries, Query: `rate(pairs_total[1h])`})
	b, err := d.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["title"] != "Sprint Velocity" {
		t.Errorf("expected title 'Sprint Velocity', got %v", result["title"])
	}
}
