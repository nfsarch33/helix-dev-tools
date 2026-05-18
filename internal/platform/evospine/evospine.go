package evospine

import "time"

// PatternEntry tracks one convergence metric observation
type PatternEntry struct {
	RecordedAt    time.Time
	PatternsTotal int
	PatternsNew   int
	Score         float64 // quality score at this observation
}

// ConvergenceStatus summarises whether patterns are stabilising
type ConvergenceStatus string

const (
	StatusConverging  ConvergenceStatus = "converging"
	StatusDiverging   ConvergenceStatus = "diverging"
	StatusStable      ConvergenceStatus = "stable"
	StatusInsufficient ConvergenceStatus = "insufficient_data"
)

// Tracker records EvoSpine pattern discovery over time
type Tracker struct {
	history []PatternEntry
}

// NewTracker creates a Tracker with no history
func NewTracker() *Tracker {
	return &Tracker{}
}

// Record adds a new observation
func (t *Tracker) Record(entry PatternEntry) {
	if entry.RecordedAt.IsZero() {
		entry.RecordedAt = time.Now()
	}
	t.history = append(t.history, entry)
}

// Convergence returns the current convergence status.
// Requires at least 2 observations; uses the trend of PatternsNew.
func (t *Tracker) Convergence() ConvergenceStatus {
	if len(t.history) < 2 {
		return StatusInsufficient
	}
	last := t.history[len(t.history)-1]
	prev := t.history[len(t.history)-2]
	switch {
	case last.PatternsNew < prev.PatternsNew:
		return StatusConverging
	case last.PatternsNew > prev.PatternsNew:
		return StatusDiverging
	default:
		return StatusStable
	}
}

// Velocity returns the average number of new patterns per observation
func (t *Tracker) Velocity() float64 {
	if len(t.history) == 0 {
		return 0
	}
	total := 0
	for _, e := range t.history {
		total += e.PatternsNew
	}
	return float64(total) / float64(len(t.history))
}

// LastN returns the last n observations (all if n >= len)
func (t *Tracker) LastN(n int) []PatternEntry {
	if n >= len(t.history) {
		return append([]PatternEntry(nil), t.history...)
	}
	return append([]PatternEntry(nil), t.history[len(t.history)-n:]...)
}
