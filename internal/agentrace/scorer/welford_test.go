package scorer

import (
	"math"
	"testing"
)

func TestWelford_OnlineMeanMatchesBatch(t *testing.T) {
	values := []float64{2, 4, 4, 4, 5, 5, 7, 9}
	var w Welford
	for _, v := range values {
		w.Update(v)
	}
	batchMean := batchMean(values)
	if math.Abs(w.Mean()-batchMean) > 1e-9 {
		t.Errorf("online mean %f != batch mean %f", w.Mean(), batchMean)
	}
}

func TestWelford_OnlineVarianceMatchesBatch(t *testing.T) {
	values := []float64{2, 4, 4, 4, 5, 5, 7, 9}
	var w Welford
	for _, v := range values {
		w.Update(v)
	}
	bv := batchVariance(values)
	if math.Abs(w.Variance()-bv) > 1e-9 {
		t.Errorf("online variance %f != batch variance %f", w.Variance(), bv)
	}
}

func TestWelford_ZeroSamples(t *testing.T) {
	var w Welford
	if w.Mean() != 0 {
		t.Errorf("expected 0 mean for empty, got %f", w.Mean())
	}
	if w.Variance() != 0 {
		t.Errorf("expected 0 variance for empty, got %f", w.Variance())
	}
	if w.StdDev() != 0 {
		t.Errorf("expected 0 stddev for empty, got %f", w.StdDev())
	}
}

func TestWelford_SingleSample(t *testing.T) {
	var w Welford
	w.Update(42.0)
	if w.Mean() != 42.0 {
		t.Errorf("expected mean 42, got %f", w.Mean())
	}
	if w.Variance() != 0 {
		t.Errorf("expected 0 variance for single sample, got %f", w.Variance())
	}
}

func batchMean(values []float64) float64 {
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func batchVariance(values []float64) float64 {
	m := batchMean(values)
	sum := 0.0
	for _, v := range values {
		sum += (v - m) * (v - m)
	}
	return sum / float64(len(values))
}
