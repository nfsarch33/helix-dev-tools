package configgate

import (
	"strconv"
	"sync"
)

type FindingKind int

const (
	FindingMissing FindingKind = iota
	FindingOutOfRange
	FindingInvalid
)

type Config struct {
	ConfigPath string
}

type Rule struct {
	Key      string
	MinValue string
	MaxValue string
	Required bool
}

type Finding struct {
	Key     string
	Kind    FindingKind
	Message string
}

type Gate struct {
	config Config
	mu     sync.Mutex
	rules  []Rule
}

func New(cfg Config) *Gate {
	return &Gate{config: cfg}
}

func (g *Gate) AddRule(r Rule) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.rules = append(g.rules, r)
}

func (g *Gate) Rules() []Rule {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]Rule, len(g.rules))
	copy(out, g.rules)
	return out
}

func (g *Gate) Check(values map[string]string) []Finding {
	g.mu.Lock()
	defer g.mu.Unlock()
	var findings []Finding
	for _, r := range g.rules {
		val, exists := values[r.Key]
		if !exists {
			if r.Required {
				findings = append(findings, Finding{Key: r.Key, Kind: FindingMissing, Message: "required key missing"})
			}
			continue
		}
		if r.MinValue != "" && r.MaxValue != "" {
			n, err := strconv.Atoi(val)
			if err == nil {
				min, _ := strconv.Atoi(r.MinValue)
				max, _ := strconv.Atoi(r.MaxValue)
				if n < min || n > max {
					findings = append(findings, Finding{Key: r.Key, Kind: FindingOutOfRange, Message: "value out of range"})
				}
			}
		}
	}
	return findings
}
