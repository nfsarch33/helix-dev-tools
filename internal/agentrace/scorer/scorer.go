package scorer

import "math"

// ZScore computes a z-score with a safety floor to prevent division by zero.
// When stddev is near zero, the denominator is clamped to max(stddev, mean*0.2, 1.0).
func ZScore(value, mean, stddev float64) float64 {
	safeStd := math.Max(stddev, math.Max(mean*0.2, 1.0))
	return (value - mean) / safeStd
}

// CompositeAnomalyScore blends tool-call z-score, duration z-score, and error
// deviation into a single anomaly signal.
//
// Weights: toolZ 30%, durationZ 30%, errorDeviation (amplified 5x) 40%.
func CompositeAnomalyScore(toolZ, durationZ, errorDeviation float64) float64 {
	return math.Abs(toolZ)*0.3 + math.Abs(durationZ)*0.3 + errorDeviation*5*0.4
}
