package domain

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Severity ranks a code-review finding.
type Severity int

const (
	SeverityNit Severity = iota
	SeverityShould
	SeverityMust
)

var severityNames = [...]string{
	SeverityNit:    "nit",
	SeverityShould: "should",
	SeverityMust:   "must",
}

func (s Severity) String() string {
	if int(s) < len(severityNames) {
		return severityNames[s]
	}
	return fmt.Sprintf("severity(%d)", int(s))
}

func (s Severity) MarshalJSON() ([]byte, error) { return json.Marshal(s.String()) }

func (s *Severity) UnmarshalJSON(b []byte) error {
	var str string
	if err := json.Unmarshal(b, &str); err != nil {
		return err
	}
	parsed, ok := ParseSeverity(str)
	if !ok {
		return fmt.Errorf("reviewd: unknown severity %q", str)
	}
	*s = parsed
	return nil
}

// ParseSeverity maps a string to a Severity, returning false if unknown.
// Accepts aliases: "critical" -> Must, "warning" -> Should, "info" -> Nit.
func ParseSeverity(s string) (Severity, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "nit", "info":
		return SeverityNit, true
	case "should", "warning":
		return SeverityShould, true
	case "must", "critical":
		return SeverityMust, true
	default:
		return SeverityNit, false
	}
}

// Finding is a single code-review observation.
type Finding struct {
	File     string   `json:"file"`
	Line     int      `json:"line,omitempty"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	Rule     string   `json:"rule,omitempty"`
}

// SeverityCounts aggregates findings by severity for the issue body.
type SeverityCounts struct {
	Must   int `json:"must"`
	Should int `json:"should"`
	Nit    int `json:"nit"`
}

// Total returns the sum of all severity counts.
func (c SeverityCounts) Total() int { return c.Must + c.Should + c.Nit }

// CountFindings tallies the severities across a slice of findings.
func CountFindings(findings []Finding) SeverityCounts {
	var c SeverityCounts
	for _, f := range findings {
		switch f.Severity {
		case SeverityMust:
			c.Must++
		case SeverityShould:
			c.Should++
		case SeverityNit:
			c.Nit++
		}
	}
	return c
}
