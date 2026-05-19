package evalharness

import (
	"testing"
	"time"
)

func TestNewHarness(t *testing.T) {
	h := New(Config{
		SprintID:    "v6310",
		AgentID:     "cursor-parent",
		SentruxPath: "/usr/local/bin/sentrux",
	})
	if h == nil {
		t.Fatal("expected non-nil harness")
	}
	if h.config.SprintID != "v6310" {
		t.Errorf("got sprint %q, want v6310", h.config.SprintID)
	}
}

func TestRecordOutcome(t *testing.T) {
	h := New(Config{SprintID: "v6310", AgentID: "test"})
	h.RecordOutcome(Outcome{
		TicketID:  "T-6310-1",
		Status:    StatusPass,
		Duration:  5 * time.Second,
		Evidence:  "commit abc123",
		TestCount: 8,
	})
	results := h.Results()
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Status != StatusPass {
		t.Errorf("got status %v, want PASS", results[0].Status)
	}
}

func TestPassRate(t *testing.T) {
	h := New(Config{SprintID: "v6310", AgentID: "test"})
	h.RecordOutcome(Outcome{TicketID: "T1", Status: StatusPass, TestCount: 5})
	h.RecordOutcome(Outcome{TicketID: "T2", Status: StatusPass, TestCount: 3})
	h.RecordOutcome(Outcome{TicketID: "T3", Status: StatusFail, TestCount: 1})

	rate := h.PassRate()
	if rate < 0.66 || rate > 0.67 {
		t.Errorf("got pass rate %f, want ~0.667", rate)
	}
}

func TestTotalTests(t *testing.T) {
	h := New(Config{SprintID: "v6310", AgentID: "test"})
	h.RecordOutcome(Outcome{TicketID: "T1", Status: StatusPass, TestCount: 5})
	h.RecordOutcome(Outcome{TicketID: "T2", Status: StatusPass, TestCount: 8})

	total := h.TotalTests()
	if total != 13 {
		t.Errorf("got %d tests, want 13", total)
	}
}

func TestSentruxDelta(t *testing.T) {
	h := New(Config{SprintID: "v6310", AgentID: "test"})
	h.SetSentruxBaseline(7013)
	h.SetSentruxCurrent(7075)

	delta := h.SentruxDelta()
	if delta != 62 {
		t.Errorf("got delta %d, want 62", delta)
	}
}

func TestGenerateReport(t *testing.T) {
	h := New(Config{SprintID: "v6310", AgentID: "cursor-parent"})
	h.RecordOutcome(Outcome{TicketID: "T1", Status: StatusPass, TestCount: 5, Duration: 3 * time.Minute})
	h.RecordOutcome(Outcome{TicketID: "T2", Status: StatusPass, TestCount: 8, Duration: 5 * time.Minute})
	h.SetSentruxBaseline(7013)
	h.SetSentruxCurrent(7075)

	report := h.GenerateReport()
	if report.SprintID != "v6310" {
		t.Errorf("report sprint %q, want v6310", report.SprintID)
	}
	if report.PassRate != 1.0 {
		t.Errorf("report pass rate %f, want 1.0", report.PassRate)
	}
	if report.TotalTests != 13 {
		t.Errorf("report total tests %d, want 13", report.TotalTests)
	}
	if report.SentruxDelta != 62 {
		t.Errorf("report sentrux delta %d, want 62", report.SentruxDelta)
	}
}
