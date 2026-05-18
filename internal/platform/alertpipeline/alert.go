package alertpipeline

import (
	"sync"
	"time"
)

// AlertSeverity is the urgency level of an alert
type AlertSeverity string

const (
	SeverityInfo     AlertSeverity = "info"
	SeverityWarning  AlertSeverity = "warning"
	SeverityCritical AlertSeverity = "critical"
)

// AlertState tracks whether an alert is open or resolved
type AlertState string

const (
	StateOpen       AlertState = "open"
	StateAcked      AlertState = "acked"
	StateResolved   AlertState = "resolved"
)

// Alert represents a fired alert
type Alert struct {
	ID         string
	RuleID     string
	Message    string
	Severity   AlertSeverity
	State      AlertState
	FiredAt    time.Time
	AckedAt    time.Time
	ResolvedAt time.Time
}

// Rule defines when an alert should fire
type Rule struct {
	ID        string
	Condition func() bool
	Message   string
	Severity  AlertSeverity
}

// Pipeline manages rules, dedup/throttling, and alert lifecycle
type Pipeline struct {
	mu       sync.Mutex
	rules    []Rule
	alerts   []Alert
	lastFire map[string]time.Time // rule_id -> last fire time
	throttle time.Duration        // min interval between fires per rule
}

// NewPipeline creates a Pipeline with the given throttle duration
func NewPipeline(throttle time.Duration) *Pipeline {
	return &Pipeline{
		lastFire: map[string]time.Time{},
		throttle: throttle,
	}
}

// AddRule registers an alert rule
func (p *Pipeline) AddRule(r Rule) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rules = append(p.rules, r)
}

// Evaluate runs all rules and fires alerts for passing conditions,
// respecting the throttle window. Returns count of new alerts fired.
func (p *Pipeline) Evaluate() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	fired := 0

	for _, rule := range p.rules {
		if !rule.Condition() {
			continue
		}
		last, ok := p.lastFire[rule.ID]
		if ok && now.Sub(last) < p.throttle {
			continue
		}
		p.lastFire[rule.ID] = now
		p.alerts = append(p.alerts, Alert{
			ID:       rule.ID + "-" + now.Format("20060102T150405"),
			RuleID:  rule.ID,
			Message: rule.Message,
			Severity: rule.Severity,
			State:    StateOpen,
			FiredAt: now,
		})
		fired++
	}
	return fired
}

// OpenAlerts returns all alerts in StateOpen
func (p *Pipeline) OpenAlerts() []Alert {
	p.mu.Lock()
	defer p.mu.Unlock()

	var open []Alert
	for _, a := range p.alerts {
		if a.State == StateOpen {
			open = append(open, a)
		}
	}
	return open
}

// Acknowledge marks an alert as acked by ID
func (p *Pipeline) Acknowledge(id string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i := range p.alerts {
		if p.alerts[i].ID == id && p.alerts[i].State == StateOpen {
			p.alerts[i].State = StateAcked
			p.alerts[i].AckedAt = time.Now()
			return true
		}
	}
	return false
}

// Resolve marks an alert as resolved by ID
func (p *Pipeline) Resolve(id string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i := range p.alerts {
		if p.alerts[i].ID == id &&
			(p.alerts[i].State == StateOpen || p.alerts[i].State == StateAcked) {
			p.alerts[i].State = StateResolved
			p.alerts[i].ResolvedAt = time.Now()
			return true
		}
	}
	return false
}
