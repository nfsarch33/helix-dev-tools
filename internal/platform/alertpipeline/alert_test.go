package alertpipeline

import (
	"testing"
	"time"
)

func TestEvaluate_FiresMatchingRule(t *testing.T) {
	p := NewPipeline(time.Hour)
	p.AddRule(Rule{ID: "r1", Condition: func() bool { return true }, Message: "test", Severity: SeverityWarning})
	n := p.Evaluate()
	if n != 1 {
		t.Errorf("expected 1 fired, got %d", n)
	}
	open := p.OpenAlerts()
	if len(open) != 1 {
		t.Errorf("expected 1 open alert, got %d", len(open))
	}
}

func TestEvaluate_SkipsFalseCondition(t *testing.T) {
	p := NewPipeline(time.Hour)
	p.AddRule(Rule{ID: "r1", Condition: func() bool { return false }, Message: "never", Severity: SeverityInfo})
	n := p.Evaluate()
	if n != 0 {
		t.Errorf("expected 0 fired, got %d", n)
	}
}

func TestEvaluate_Throttle_PreventsDuplicate(t *testing.T) {
	p := NewPipeline(time.Hour)
	p.AddRule(Rule{ID: "r1", Condition: func() bool { return true }, Message: "test", Severity: SeverityWarning})
	p.Evaluate()
	n := p.Evaluate() // second call within throttle window
	if n != 0 {
		t.Errorf("expected 0 (throttled), got %d", n)
	}
}

func TestEvaluate_NoThrottle_AllowsRepeat(t *testing.T) {
	p := NewPipeline(0)
	p.AddRule(Rule{ID: "r1", Condition: func() bool { return true }, Message: "test", Severity: SeverityWarning})
	p.Evaluate()
	n := p.Evaluate()
	if n != 1 {
		t.Errorf("expected 1 with zero throttle, got %d", n)
	}
}

func TestAcknowledge_TransitionsState(t *testing.T) {
	p := NewPipeline(0)
	p.AddRule(Rule{ID: "r1", Condition: func() bool { return true }, Message: "test", Severity: SeverityInfo})
	p.Evaluate()
	open := p.OpenAlerts()
	if len(open) == 0 {
		t.Fatal("no open alerts to ack")
	}
	ok := p.Acknowledge(open[0].ID)
	if !ok {
		t.Error("expected Acknowledge to return true")
	}
	if len(p.OpenAlerts()) != 0 {
		t.Error("expected no open alerts after ack")
	}
}

func TestResolve_TransitionsState(t *testing.T) {
	p := NewPipeline(0)
	p.AddRule(Rule{ID: "r1", Condition: func() bool { return true }, Message: "test", Severity: SeverityCritical})
	p.Evaluate()
	open := p.OpenAlerts()
	ok := p.Resolve(open[0].ID)
	if !ok {
		t.Error("expected Resolve to return true")
	}
	if len(p.OpenAlerts()) != 0 {
		t.Error("expected no open alerts after resolve")
	}
}

func TestResolve_NotFound(t *testing.T) {
	p := NewPipeline(0)
	ok := p.Resolve("nonexistent-id")
	if ok {
		t.Error("expected false for nonexistent alert")
	}
}
