package experiment

import "time"

// Status tracks the lifecycle of an experiment
type Status string

const (
	StatusPending  Status = "pending"
	StatusRunning  Status = "running"
	StatusComplete Status = "complete"
	StatusAborted  Status = "aborted"
)

// Outcome records what happened when the experiment ran
type Outcome struct {
	MetricBefore float64
	MetricAfter  float64
	Kept         bool
}

// Delta returns the absolute metric improvement
func (o Outcome) Delta() float64 {
	return o.MetricAfter - o.MetricBefore
}

// Experiment holds a hypothesis and its outcome
type Experiment struct {
	ID         string
	Hypothesis string
	Mutation   string
	Metric     string
	BudgetMS   int64
	Status     Status
	StartedAt  time.Time
	EndedAt    time.Time
	Outcome    *Outcome
}

// Registry stores experiment history
type Registry struct {
	experiments []Experiment
}

// NewRegistry creates an empty registry
func NewRegistry() *Registry {
	return &Registry{}
}

// Add inserts an experiment; sets StartedAt if zero
func (r *Registry) Add(e Experiment) {
	if e.StartedAt.IsZero() {
		e.StartedAt = time.Now()
	}
	r.experiments = append(r.experiments, e)
}

// Get returns the experiment with the given ID, or false
func (r *Registry) Get(id string) (Experiment, bool) {
	for _, e := range r.experiments {
		if e.ID == id {
			return e, true
		}
	}
	return Experiment{}, false
}

// Complete records an outcome for an existing experiment
func (r *Registry) Complete(id string, outcome Outcome) bool {
	for i := range r.experiments {
		if r.experiments[i].ID == id {
			r.experiments[i].Status = StatusComplete
			r.experiments[i].EndedAt = time.Now()
			r.experiments[i].Outcome = &outcome
			return true
		}
	}
	return false
}

// All returns a copy of all experiments
func (r *Registry) All() []Experiment {
	result := make([]Experiment, len(r.experiments))
	copy(result, r.experiments)
	return result
}

// KeptCount returns how many experiments were kept
func (r *Registry) KeptCount() int {
	count := 0
	for _, e := range r.experiments {
		if e.Outcome != nil && e.Outcome.Kept {
			count++
		}
	}
	return count
}
