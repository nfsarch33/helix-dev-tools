package reportformat

import (
	"strings"
	"testing"
	"time"
)

func TestNewReport(t *testing.T) {
	r := NewReport("v6420 Daily Report", time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC))
	if r.Title != "v6420 Daily Report" {
		t.Errorf("title: %s", r.Title)
	}
}

func TestAddSection(t *testing.T) {
	r := NewReport("test", time.Now())
	r.AddSection("Packages Delivered", []string{"tokentrack", "embedwarmup", "keyrotate"})
	if len(r.Sections) != 1 {
		t.Fatalf("sections: %d", len(r.Sections))
	}
	if len(r.Sections[0].Items) != 3 {
		t.Errorf("items: %d", len(r.Sections[0].Items))
	}
}

func TestAddMetric(t *testing.T) {
	r := NewReport("test", time.Now())
	r.AddMetric("Tests Passed", "86")
	r.AddMetric("Coverage", "92%")
	if len(r.Metrics) != 2 {
		t.Errorf("metrics: %d", len(r.Metrics))
	}
}

func TestRenderMarkdown(t *testing.T) {
	r := NewReport("Sprint v6420", time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC))
	r.AddSection("Delivered", []string{"agentlifecycle", "reportformat"})
	r.AddMetric("Packages", "5")
	r.AddNote("All tests green with -race.")

	md := r.RenderMarkdown()
	if !strings.Contains(md, "# Sprint v6420") {
		t.Error("missing title")
	}
	if !strings.Contains(md, "## Delivered") {
		t.Error("missing section header")
	}
	if !strings.Contains(md, "- agentlifecycle") {
		t.Error("missing item")
	}
	if !strings.Contains(md, "| Packages | 5 |") {
		t.Error("missing metric row")
	}
	if !strings.Contains(md, "All tests green") {
		t.Error("missing note")
	}
}

func TestRenderEmpty(t *testing.T) {
	r := NewReport("Empty", time.Now())
	md := r.RenderMarkdown()
	if !strings.Contains(md, "# Empty") {
		t.Error("missing title in empty report")
	}
}

func TestAddNote(t *testing.T) {
	r := NewReport("test", time.Now())
	r.AddNote("first note")
	r.AddNote("second note")
	if len(r.Notes) != 2 {
		t.Errorf("notes: %d", len(r.Notes))
	}
}

func TestRenderIncludesDate(t *testing.T) {
	d := time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)
	r := NewReport("test", d)
	md := r.RenderMarkdown()
	if !strings.Contains(md, "2026-05-20") {
		t.Error("missing date in render")
	}
}

func TestMultipleSections(t *testing.T) {
	r := NewReport("multi", time.Now())
	r.AddSection("Alpha", []string{"a1", "a2"})
	r.AddSection("Beta", []string{"b1"})
	md := r.RenderMarkdown()
	if !strings.Contains(md, "## Alpha") || !strings.Contains(md, "## Beta") {
		t.Error("missing multiple section headers")
	}
}
