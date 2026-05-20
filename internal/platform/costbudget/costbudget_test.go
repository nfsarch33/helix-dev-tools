package costbudget

import (
	"testing"
	"time"
)

func TestNewBudget(t *testing.T) {
	b := NewBudget(10.0, PeriodDaily)
	if b.Limit != 10.0 {
		t.Errorf("limit: %f", b.Limit)
	}
	if b.Period != PeriodDaily {
		t.Errorf("period: %s", b.Period)
	}
}

func TestRecordSpend(t *testing.T) {
	b := NewBudget(10.0, PeriodDaily)
	b.RecordSpend(2.50, "model-a")
	b.RecordSpend(1.50, "model-b")
	if b.Spent != 4.0 {
		t.Errorf("spent: %f", b.Spent)
	}
}

func TestRemaining(t *testing.T) {
	b := NewBudget(10.0, PeriodDaily)
	b.RecordSpend(3.0, "model-a")
	if b.Remaining() != 7.0 {
		t.Errorf("remaining: %f", b.Remaining())
	}
}

func TestExceeded(t *testing.T) {
	b := NewBudget(5.0, PeriodDaily)
	b.RecordSpend(5.01, "model-a")
	if !b.Exceeded() {
		t.Error("should be exceeded")
	}
}

func TestNotExceeded(t *testing.T) {
	b := NewBudget(5.0, PeriodDaily)
	b.RecordSpend(4.99, "model-a")
	if b.Exceeded() {
		t.Error("should not be exceeded")
	}
}

func TestAlertThresholds(t *testing.T) {
	b := NewBudget(10.0, PeriodDaily)
	b.SetAlertThresholds(0.5, 0.8, 0.95)

	b.RecordSpend(4.0, "model-a")
	alerts := b.CheckAlerts()
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts at 40%%, got %d", len(alerts))
	}

	b.RecordSpend(2.0, "model-a")
	alerts = b.CheckAlerts()
	if len(alerts) != 1 {
		t.Errorf("expected 1 alert at 60%%, got %d", len(alerts))
	}
}

func TestAlertAt80(t *testing.T) {
	b := NewBudget(10.0, PeriodDaily)
	b.SetAlertThresholds(0.5, 0.8, 0.95)
	b.RecordSpend(8.5, "model-a")
	alerts := b.CheckAlerts()
	if len(alerts) != 2 {
		t.Errorf("expected 2 alerts at 85%%, got %d", len(alerts))
	}
}

func TestAlertAt95(t *testing.T) {
	b := NewBudget(10.0, PeriodDaily)
	b.SetAlertThresholds(0.5, 0.8, 0.95)
	b.RecordSpend(9.6, "model-a")
	alerts := b.CheckAlerts()
	if len(alerts) != 3 {
		t.Errorf("expected 3 alerts at 96%%, got %d", len(alerts))
	}
}

func TestPerModelSpend(t *testing.T) {
	b := NewBudget(20.0, PeriodWeekly)
	b.RecordSpend(3.0, "model-a")
	b.RecordSpend(5.0, "model-b")
	b.RecordSpend(2.0, "model-a")
	byModel := b.SpendByModel()
	if byModel["model-a"] != 5.0 {
		t.Errorf("model-a: %f", byModel["model-a"])
	}
	if byModel["model-b"] != 5.0 {
		t.Errorf("model-b: %f", byModel["model-b"])
	}
}

func TestReset(t *testing.T) {
	b := NewBudget(10.0, PeriodDaily)
	b.RecordSpend(7.0, "model-a")
	b.Reset()
	if b.Spent != 0 {
		t.Errorf("spent after reset: %f", b.Spent)
	}
	if len(b.SpendByModel()) != 0 {
		t.Error("by-model should be empty after reset")
	}
}

func TestPeriodLabel(t *testing.T) {
	tests := []struct {
		p    Period
		want string
	}{
		{PeriodDaily, "daily"},
		{PeriodWeekly, "weekly"},
		{PeriodMonthly, "monthly"},
	}
	for _, tt := range tests {
		if string(tt.p) != tt.want {
			t.Errorf("period %q != %q", tt.p, tt.want)
		}
	}
}

func TestStartedAt(t *testing.T) {
	before := time.Now()
	b := NewBudget(10.0, PeriodDaily)
	after := time.Now()
	if b.StartedAt.Before(before) || b.StartedAt.After(after) {
		t.Errorf("started_at out of range")
	}
}
