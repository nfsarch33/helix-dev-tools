package policybundle

import (
	"errors"
	"fmt"
	"regexp"
	"sync"
)

type Severity string

const (
	SeverityBlock Severity = "block"
	SeverityWarn  Severity = "warn"
	SeverityInfo  Severity = "info"
)

var validSeverities = map[Severity]bool{
	SeverityBlock: true,
	SeverityWarn:  true,
	SeverityInfo:  true,
}

type Rule struct {
	ID          string `json:"id"`
	Pattern     string `json:"pattern"`
	Description string `json:"description,omitempty"`
}

type Bundle struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Severity    Severity `json:"severity"`
	Rules       []Rule   `json:"rules,omitempty"`
	Enabled     bool     `json:"enabled"`
}

type Violation struct {
	BundleID string `json:"bundle_id"`
	RuleID   string `json:"rule_id"`
	Pattern  string `json:"pattern"`
	Match    string `json:"match"`
}

type BundleResult struct {
	BundleID   string      `json:"bundle_id"`
	Severity   Severity    `json:"severity"`
	Violations []Violation `json:"violations"`
}

type Registry struct {
	mu      sync.RWMutex
	bundles map[string]Bundle
}

func New() *Registry {
	return &Registry{
		bundles: make(map[string]Bundle),
	}
}

func Validate(b Bundle) error {
	if b.ID == "" {
		return errors.New("bundle ID is required")
	}
	if b.Name == "" {
		return errors.New("bundle Name is required")
	}
	if !validSeverities[b.Severity] {
		return fmt.Errorf("invalid severity %q", b.Severity)
	}
	for _, r := range b.Rules {
		if _, err := regexp.Compile(r.Pattern); err != nil {
			return fmt.Errorf("rule %q has invalid pattern %q: %w", r.ID, r.Pattern, err)
		}
	}
	return nil
}

func (reg *Registry) Register(b Bundle) error {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	if _, exists := reg.bundles[b.ID]; exists {
		return fmt.Errorf("bundle %q already registered", b.ID)
	}

	reg.bundles[b.ID] = b
	return nil
}

func (reg *Registry) Get(id string) (Bundle, error) {
	reg.mu.RLock()
	defer reg.mu.RUnlock()

	b, exists := reg.bundles[id]
	if !exists {
		return Bundle{}, fmt.Errorf("bundle %q not found", id)
	}
	return b, nil
}

func (reg *Registry) ListBySeverity(sev Severity) []Bundle {
	reg.mu.RLock()
	defer reg.mu.RUnlock()

	var result []Bundle
	for _, b := range reg.bundles {
		if b.Severity == sev {
			result = append(result, b)
		}
	}
	return result
}

func (reg *Registry) ListEnabled() []Bundle {
	reg.mu.RLock()
	defer reg.mu.RUnlock()

	var result []Bundle
	for _, b := range reg.bundles {
		if b.Enabled {
			result = append(result, b)
		}
	}
	return result
}

func (reg *Registry) All() []Bundle {
	reg.mu.RLock()
	defer reg.mu.RUnlock()

	result := make([]Bundle, 0, len(reg.bundles))
	for _, b := range reg.bundles {
		result = append(result, b)
	}
	return result
}

func (reg *Registry) Evaluate(bundleID string, input string) []Violation {
	reg.mu.RLock()
	b, exists := reg.bundles[bundleID]
	reg.mu.RUnlock()

	if !exists {
		return nil
	}

	var violations []Violation
	for _, rule := range b.Rules {
		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			continue
		}
		match := re.FindString(input)
		if match != "" {
			violations = append(violations, Violation{
				BundleID: bundleID,
				RuleID:   rule.ID,
				Pattern:  rule.Pattern,
				Match:    match,
			})
		}
	}
	return violations
}

func (reg *Registry) EvaluateAll(input string) []BundleResult {
	reg.mu.RLock()
	bundles := make([]Bundle, 0, len(reg.bundles))
	for _, b := range reg.bundles {
		bundles = append(bundles, b)
	}
	reg.mu.RUnlock()

	var results []BundleResult
	for _, b := range bundles {
		violations := reg.Evaluate(b.ID, input)
		if len(violations) > 0 {
			results = append(results, BundleResult{
				BundleID:   b.ID,
				Severity:   b.Severity,
				Violations: violations,
			})
		}
	}
	return results
}
