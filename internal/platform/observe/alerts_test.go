package observe

import "testing"

func TestAlertEngine_FireAboveThreshold(t *testing.T) {
	e := NewAlertEngine()
	e.AddRule(AlertRule{Name: "high-latency", Metric: "latency_ms", Threshold: 100, Condition: "above", Severity: SeverityWarning})

	fired := e.Evaluate(map[string]float64{"latency_ms": 150})
	if len(fired) != 1 {
		t.Errorf("expected 1 alert, got %d", len(fired))
	}
}

func TestAlertEngine_NoFireBelowThreshold(t *testing.T) {
	e := NewAlertEngine()
	e.AddRule(AlertRule{Name: "high-latency", Metric: "latency_ms", Threshold: 100, Condition: "above"})

	fired := e.Evaluate(map[string]float64{"latency_ms": 50})
	if len(fired) != 0 {
		t.Error("should not fire below threshold")
	}
}

func TestAlertEngine_BelowCondition(t *testing.T) {
	e := NewAlertEngine()
	e.AddRule(AlertRule{Name: "low-memory", Metric: "free_pct", Threshold: 5, Condition: "below", Severity: SeverityCritical})

	fired := e.Evaluate(map[string]float64{"free_pct": 3})
	if len(fired) != 1 {
		t.Error("expected alert for below-threshold condition")
	}
}

func TestAlertEngine_MissingMetric(t *testing.T) {
	e := NewAlertEngine()
	e.AddRule(AlertRule{Name: "x", Metric: "missing", Threshold: 0, Condition: "above"})

	fired := e.Evaluate(map[string]float64{"other": 100})
	if len(fired) != 0 {
		t.Error("should not fire for missing metric")
	}
}

func TestAlertEngine_FiredHistory(t *testing.T) {
	e := NewAlertEngine()
	e.AddRule(AlertRule{Name: "r", Metric: "m", Threshold: 0, Condition: "above"})
	e.Evaluate(map[string]float64{"m": 1})
	e.Evaluate(map[string]float64{"m": 2})

	if len(e.FiredAlerts()) != 2 {
		t.Errorf("expected 2 in history, got %d", len(e.FiredAlerts()))
	}
}

func TestAlertEngine_RuleCount(t *testing.T) {
	e := NewAlertEngine()
	e.AddRule(AlertRule{Name: "a"})
	e.AddRule(AlertRule{Name: "b"})
	if e.RuleCount() != 2 {
		t.Errorf("expected 2 rules")
	}
}
