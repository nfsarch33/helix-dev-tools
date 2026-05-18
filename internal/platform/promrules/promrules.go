package promrules

import "fmt"

// Severity of an alert rule
type Severity string

const (
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
	SeverityInfo     Severity = "info"
)

// AlertRule defines when a Prometheus alert should fire
type AlertRule struct {
	Name      string
	Expr      string
	ForDur    string
	Severity  Severity
	Summary   string
}

// RuleSet is a named collection of alert rules
type RuleSet struct {
	Name  string
	rules []AlertRule
}

// NewRuleSet creates an empty rule set
func NewRuleSet(name string) *RuleSet {
	return &RuleSet{Name: name}
}

// Add appends a rule to the set
func (rs *RuleSet) Add(r AlertRule) {
	rs.rules = append(rs.rules, r)
}

// Get returns the rule with the given name, or false
func (rs *RuleSet) Get(name string) (AlertRule, bool) {
	for _, r := range rs.rules {
		if r.Name == name {
			return r, true
		}
	}
	return AlertRule{}, false
}

// BySeverity returns all rules with the given severity
func (rs *RuleSet) BySeverity(s Severity) []AlertRule {
	var result []AlertRule
	for _, r := range rs.rules {
		if r.Severity == s {
			result = append(result, r)
		}
	}
	return result
}

// Validate returns errors for rules missing Name or Expr
func (rs *RuleSet) Validate() []error {
	var errs []error
	for _, r := range rs.rules {
		if r.Name == "" {
			errs = append(errs, fmt.Errorf("rule missing name"))
		}
		if r.Expr == "" {
			errs = append(errs, fmt.Errorf("rule %q missing expression", r.Name))
		}
	}
	return errs
}

// All returns a copy of all rules
func (rs *RuleSet) All() []AlertRule {
	result := make([]AlertRule, len(rs.rules))
	copy(result, rs.rules)
	return result
}
