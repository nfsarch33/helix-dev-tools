package evospinevis

import "time"

// PatternEvent records when a pattern was discovered or retired
type PatternEvent struct {
	PatternID  string
	Action     string // "discovered" or "retired"
	Impact     float64
	RecordedAt time.Time
}

// Timeline holds the ordered history of pattern events
type Timeline struct {
	events []PatternEvent
}

// NewTimeline creates an empty timeline
func NewTimeline() *Timeline {
	return &Timeline{}
}

// Record appends a pattern event
func (tl *Timeline) Record(e PatternEvent) {
	if e.RecordedAt.IsZero() {
		e.RecordedAt = time.Now()
	}
	tl.events = append(tl.events, e)
}

// AllEvents returns a copy of all events
func (tl *Timeline) AllEvents() []PatternEvent {
	result := make([]PatternEvent, len(tl.events))
	copy(result, tl.events)
	return result
}

// ActivePatterns returns IDs of patterns that were discovered but not retired
func (tl *Timeline) ActivePatterns() []string {
	discovered := map[string]bool{}
	for _, e := range tl.events {
		switch e.Action {
		case "discovered":
			discovered[e.PatternID] = true
		case "retired":
			delete(discovered, e.PatternID)
		}
	}
	var result []string
	for id := range discovered {
		result = append(result, id)
	}
	return result
}

// AverageImpact returns the mean impact of discovered patterns (0 if none)
func (tl *Timeline) AverageImpact() float64 {
	total := 0.0
	count := 0
	for _, e := range tl.events {
		if e.Action == "discovered" {
			total += e.Impact
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return total / float64(count)
}
