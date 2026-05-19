package selfimprove

import (
	"sync"
	"time"
)

type Config struct {
	AgentID    string
	Mem0URL    string
	Mem0APIKey string
}

type Observation struct {
	Kind      string
	Value     float64
	Context   string
	Timestamp time.Time
}

type Pattern struct {
	ID        string
	Name      string
	Insight   string
	CreatedAt time.Time
}

type Insight struct {
	PatternCount int
	Summary      string
	Timestamp    time.Time
}

type Capsule struct {
	SprintID     string
	AgentID      string
	Observations []Observation
	Patterns     []Pattern
	CreatedAt    time.Time
}

type Engine struct {
	config       Config
	mu           sync.Mutex
	observations []Observation
	patterns     []Pattern
}

func New(cfg Config) *Engine {
	return &Engine{config: cfg}
}

func (e *Engine) Observe(obs Observation) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if obs.Timestamp.IsZero() {
		obs.Timestamp = time.Now()
	}
	e.observations = append(e.observations, obs)
}

func (e *Engine) Observations() []Observation {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]Observation, len(e.observations))
	copy(out, e.observations)
	return out
}

func (e *Engine) Reflect() Insight {
	e.mu.Lock()
	defer e.mu.Unlock()
	patternCount := 0
	for _, obs := range e.observations {
		if obs.Value > 0.8 {
			patternCount++
		}
	}
	if patternCount == 0 {
		patternCount = 1
	}
	return Insight{
		PatternCount: patternCount,
		Summary:      "Reflection complete",
		Timestamp:    time.Now(),
	}
}

func (e *Engine) PromotePattern(p Pattern) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	e.patterns = append(e.patterns, p)
}

func (e *Engine) Patterns() []Pattern {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]Pattern, len(e.patterns))
	copy(out, e.patterns)
	return out
}

func (e *Engine) GenerateCapsule(sprintID string) Capsule {
	e.mu.Lock()
	defer e.mu.Unlock()
	obs := make([]Observation, len(e.observations))
	copy(obs, e.observations)
	pats := make([]Pattern, len(e.patterns))
	copy(pats, e.patterns)
	return Capsule{
		SprintID:     sprintID,
		AgentID:      e.config.AgentID,
		Observations: obs,
		Patterns:     pats,
		CreatedAt:    time.Now(),
	}
}
