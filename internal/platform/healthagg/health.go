package healthagg

import "time"

// ComponentState describes a single component's health
type ComponentState struct {
	Name    string
	Healthy bool
	Weight  int // contribution to overall score (default 1)
}

// HealthReport is the aggregated health picture
type HealthReport struct {
	Score      int // 0-100
	Healthy    int
	Total      int
	Components []ComponentState
	Trend      string // "improving", "degrading", "stable"
	RecordedAt time.Time
}

// Aggregator computes overall health from component states
type Aggregator struct {
	history []HealthReport
}

// NewAggregator creates an empty Aggregator
func NewAggregator() *Aggregator {
	return &Aggregator{}
}

// Aggregate computes a HealthReport from the given component states
func (a *Aggregator) Aggregate(components []ComponentState) HealthReport {
	if len(components) == 0 {
		report := HealthReport{Score: 0, Total: 0, RecordedAt: time.Now()}
		a.history = append(a.history, report)
		return report
	}

	totalWeight := 0
	healthyWeight := 0
	healthyCount := 0

	for _, c := range components {
		w := c.Weight
		if w <= 0 {
			w = 1
		}
		totalWeight += w
		if c.Healthy {
			healthyWeight += w
			healthyCount++
		}
	}

	score := 0
	if totalWeight > 0 {
		score = healthyWeight * 100 / totalWeight
	}

	report := HealthReport{
		Score:      score,
		Healthy:    healthyCount,
		Total:      len(components),
		Components: components,
		Trend:      a.computeTrend(score),
		RecordedAt: time.Now(),
	}
	a.history = append(a.history, report)
	return report
}

func (a *Aggregator) computeTrend(currentScore int) string {
	n := len(a.history)
	if n == 0 {
		return "stable"
	}
	prev := a.history[n-1].Score
	if currentScore > prev+5 {
		return "improving"
	}
	if currentScore < prev-5 {
		return "degrading"
	}
	return "stable"
}

// LastN returns the last N health reports
func (a *Aggregator) LastN(n int) []HealthReport {
	if n >= len(a.history) {
		return a.history
	}
	return a.history[len(a.history)-n:]
}
