package agenttrace

import "testing"

func TestRecord_Stats(t *testing.T) {
	tr := NewTracer()
	tr.Record(Event{SessionID: "s1", Tool: "edit", DurationMS: 100})
	tr.Record(Event{SessionID: "s1", Tool: "test", DurationMS: 500})
	tr.Record(Event{SessionID: "s2", Tool: "edit", DurationMS: 50}) // different session

	stats := tr.Stats("s1")
	if stats.EventCount != 2 {
		t.Errorf("expected 2 events, got %d", stats.EventCount)
	}
	if stats.TotalMS != 600 {
		t.Errorf("expected 600ms total, got %d", stats.TotalMS)
	}
	if stats.ToolCounts["edit"] != 1 {
		t.Errorf("expected 1 edit call, got %d", stats.ToolCounts["edit"])
	}
}

func TestStats_ErrorCount(t *testing.T) {
	tr := NewTracer()
	tr.Record(Event{SessionID: "s1", Tool: "edit", Error: false})
	tr.Record(Event{SessionID: "s1", Tool: "shell", Error: true})
	tr.Record(Event{SessionID: "s1", Tool: "shell", Error: true})
	stats := tr.Stats("s1")
	if stats.ErrorCount != 2 {
		t.Errorf("expected 2 errors, got %d", stats.ErrorCount)
	}
}

func TestTopTools(t *testing.T) {
	tr := NewTracer()
	for i := 0; i < 5; i++ {
		tr.Record(Event{Tool: "edit"})
	}
	for i := 0; i < 3; i++ {
		tr.Record(Event{Tool: "read"})
	}
	tr.Record(Event{Tool: "shell"})

	top := tr.TopTools(2)
	if len(top) != 2 {
		t.Fatalf("expected 2 top tools, got %d", len(top))
	}
	if top[0] != "edit" {
		t.Errorf("expected edit as most-used, got %s", top[0])
	}
}

func TestErrorRate(t *testing.T) {
	tr := NewTracer()
	tr.Record(Event{Error: false})
	tr.Record(Event{Error: false})
	tr.Record(Event{Error: true})
	tr.Record(Event{Error: true})
	rate := tr.ErrorRate()
	if rate < 0.49 || rate > 0.51 {
		t.Errorf("expected error rate ~0.5, got %f", rate)
	}
}

func TestErrorRate_Empty(t *testing.T) {
	tr := NewTracer()
	if tr.ErrorRate() != 0 {
		t.Error("expected 0 error rate for empty tracer")
	}
}
