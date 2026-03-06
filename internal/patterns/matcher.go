package patterns

import (
	"fmt"
	"regexp"
	"strings"
)

// Action represents the result of pattern matching.
type Action int

const (
	ActionAllow Action = iota
	ActionWarn
	ActionDeny
)

// compiledRule pairs a compiled regex with its source string for logging.
type compiledRule struct {
	pattern *regexp.Regexp
	source  string
}

// Matcher holds pre-compiled deny and warn regex patterns.
// Thread-safe after construction (read-only).
type Matcher struct {
	deny []compiledRule
	warn []compiledRule
}

// NewMatcher compiles all deny and warn patterns once.
// Returns an error if any pattern fails to compile.
func NewMatcher(denyPatterns, warnPatterns []string) (*Matcher, error) {
	m := &Matcher{}
	for _, p := range denyPatterns {
		re, err := regexp.Compile("(?i)" + p)
		if err != nil {
			return nil, fmt.Errorf("compile deny pattern %q: %w", p, err)
		}
		m.deny = append(m.deny, compiledRule{pattern: re, source: p})
	}
	for _, p := range warnPatterns {
		re, err := regexp.Compile("(?i)" + p)
		if err != nil {
			return nil, fmt.Errorf("compile warn pattern %q: %w", p, err)
		}
		m.warn = append(m.warn, compiledRule{pattern: re, source: p})
	}
	return m, nil
}

// Match checks input against deny patterns first, then warn patterns.
// Returns the action and the matched pattern source string.
func (m *Matcher) Match(input string) (Action, string) {
	for _, r := range m.deny {
		if r.pattern.MatchString(input) {
			src := r.source
			if len(src) > 30 {
				src = src[:30]
			}
			return ActionDeny, src
		}
	}
	for _, r := range m.warn {
		if r.pattern.MatchString(input) {
			return ActionWarn, r.source
		}
	}
	return ActionAllow, ""
}

// MatchExact checks if the lowercased input exactly matches any pattern in the list.
func MatchExact(input string, patterns []string) bool {
	lower := strings.ToLower(input)
	for _, p := range patterns {
		if lower == p {
			return true
		}
	}
	return false
}

// ContainsAny checks if the input contains any of the given substrings.
func ContainsAny(input string, substrings []string) bool {
	for _, s := range substrings {
		if strings.Contains(input, s) {
			return true
		}
	}
	return false
}
