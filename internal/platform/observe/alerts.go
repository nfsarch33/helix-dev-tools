package observe

import (
	"fmt"
	"sync"
	"time"
)

type AlertSeverity int

const (
	SeverityInfo AlertSeverity = iota
	SeverityWarning
	SeverityCritical
)

type AlertRule struct {
	Name      string
	Metric    string
	Threshold float64
	Severity  AlertSeverity
	Condition string
}

type FiredAlert struct {
	Rule     AlertRule
	Value    float64
	FiredAt  time.Time
	Resolved bool
}

type AlertEngine struct {
	mu    sync.RWMutex
	rules []AlertRule
	fired []FiredAlert
}

func NewAlertEngine() *AlertEngine {
	return &AlertEngine{}
}

func (e *AlertEngine) AddRule(rule AlertRule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rules = append(e.rules, rule)
}

func (e *AlertEngine) Evaluate(metrics map[string]float64) []FiredAlert {
	e.mu.Lock()
	defer e.mu.Unlock()

	var newAlerts []FiredAlert
	for _, rule := range e.rules {
		val, ok := metrics[rule.Metric]
		if !ok {
			continue
		}
		if e.shouldFire(rule, val) {
			alert := FiredAlert{Rule: rule, Value: val, FiredAt: time.Now()}
			newAlerts = append(newAlerts, alert)
			e.fired = append(e.fired, alert)
		}
	}
	return newAlerts
}

func (e *AlertEngine) shouldFire(rule AlertRule, value float64) bool {
	switch rule.Condition {
	case "above":
		return value > rule.Threshold
	case "below":
		return value < rule.Threshold
	case "equal":
		return value == rule.Threshold
	default:
		return value > rule.Threshold
	}
}

func (e *AlertEngine) FiredAlerts() []FiredAlert {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]FiredAlert, len(e.fired))
	copy(result, e.fired)
	return result
}

func (e *AlertEngine) RuleCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.rules)
}

func (e *AlertEngine) Summary() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return fmt.Sprintf("%d rules, %d fired", len(e.rules), len(e.fired))
}
