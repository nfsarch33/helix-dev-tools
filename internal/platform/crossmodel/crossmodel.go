package crossmodel

import "time"

// ModelTier classifies a model by capability/cost
type ModelTier string

const (
	TierFast     ModelTier = "fast"
	TierBalanced ModelTier = "balanced"
	TierCapable  ModelTier = "capable"
)

// EvalResult holds one model's evaluation run outcome
type EvalResult struct {
	ModelID    string
	Tier       ModelTier
	Score      float64
	LatencyMS  int64
	CostUSD    float64
	RunAt      time.Time
}

// Comparison holds results from evaluating multiple models on the same task
type Comparison struct {
	TaskID  string
	Results []EvalResult
}

// NewComparison creates a Comparison for a given task
func NewComparison(taskID string) *Comparison {
	return &Comparison{TaskID: taskID}
}

// Add appends a model eval result
func (c *Comparison) Add(r EvalResult) {
	if r.RunAt.IsZero() {
		r.RunAt = time.Now()
	}
	c.Results = append(c.Results, r)
}

// Best returns the result with the highest score (ties broken by lower latency)
func (c *Comparison) Best() (EvalResult, bool) {
	if len(c.Results) == 0 {
		return EvalResult{}, false
	}
	best := c.Results[0]
	for _, r := range c.Results[1:] {
		if r.Score > best.Score || (r.Score == best.Score && r.LatencyMS < best.LatencyMS) {
			best = r
		}
	}
	return best, true
}

// Recommend returns the cheapest model within scoreThreshold of the best score
func (c *Comparison) Recommend(scoreThreshold float64) (EvalResult, bool) {
	best, ok := c.Best()
	if !ok {
		return EvalResult{}, false
	}
	floor := best.Score - scoreThreshold
	var candidates []EvalResult
	for _, r := range c.Results {
		if r.Score >= floor {
			candidates = append(candidates, r)
		}
	}
	if len(candidates) == 0 {
		return best, true
	}
	rec := candidates[0]
	for _, r := range candidates[1:] {
		if r.CostUSD < rec.CostUSD {
			rec = r
		}
	}
	return rec, true
}
