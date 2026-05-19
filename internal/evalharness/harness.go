package evalharness

import (
	"sync"
	"time"
)

type Status int

const (
	StatusPass Status = iota
	StatusFail
	StatusSkip
)

type Config struct {
	SprintID    string
	AgentID     string
	SentruxPath string
}

type Outcome struct {
	TicketID  string
	Status    Status
	Duration  time.Duration
	Evidence  string
	TestCount int
	Error     string
}

type Report struct {
	SprintID      string
	AgentID       string
	PassRate      float64
	TotalTests    int
	TotalDuration time.Duration
	SentruxDelta  int
	Outcomes      []Outcome
}

type Harness struct {
	config           Config
	mu               sync.Mutex
	outcomes         []Outcome
	sentruxBaseline  int
	sentruxCurrent   int
}

func New(cfg Config) *Harness {
	return &Harness{config: cfg}
}

func (h *Harness) RecordOutcome(o Outcome) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.outcomes = append(h.outcomes, o)
}

func (h *Harness) Results() []Outcome {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]Outcome, len(h.outcomes))
	copy(out, h.outcomes)
	return out
}

func (h *Harness) PassRate() float64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.passRateLocked()
}

func (h *Harness) passRateLocked() float64 {
	if len(h.outcomes) == 0 {
		return 0
	}
	passed := 0
	for _, o := range h.outcomes {
		if o.Status == StatusPass {
			passed++
		}
	}
	return float64(passed) / float64(len(h.outcomes))
}

func (h *Harness) TotalTests() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.totalTestsLocked()
}

func (h *Harness) totalTestsLocked() int {
	total := 0
	for _, o := range h.outcomes {
		total += o.TestCount
	}
	return total
}

func (h *Harness) SetSentruxBaseline(score int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sentruxBaseline = score
}

func (h *Harness) SetSentruxCurrent(score int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sentruxCurrent = score
}

func (h *Harness) SentruxDelta() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.sentruxCurrent - h.sentruxBaseline
}

func (h *Harness) GenerateReport() Report {
	h.mu.Lock()
	defer h.mu.Unlock()
	var totalDur time.Duration
	for _, o := range h.outcomes {
		totalDur += o.Duration
	}
	outcomes := make([]Outcome, len(h.outcomes))
	copy(outcomes, h.outcomes)
	return Report{
		SprintID:      h.config.SprintID,
		AgentID:       h.config.AgentID,
		PassRate:      h.passRateLocked(),
		TotalTests:    h.totalTestsLocked(),
		TotalDuration: totalDur,
		SentruxDelta:  h.sentruxCurrent - h.sentruxBaseline,
		Outcomes:      outcomes,
	}
}
