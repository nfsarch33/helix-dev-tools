package scorer

import (
	"math"
	"testing"
)

func TestZScore_HighDurationFlagsAnomaly(t *testing.T) {
	var w Welford
	for _, v := range []float64{100, 110, 105, 95, 100} {
		w.Update(v)
	}
	z := ZScore(500, w.Mean(), w.StdDev())
	if z < 2.0 {
		t.Errorf("expected z >= 2.0 for outlier 500ms against ~100ms baseline, got %f", z)
	}
}

func TestZScore_SafetyFloorAvoidsDivByZero(t *testing.T) {
	z := ZScore(10.0, 0.0, 0.0)
	if math.IsInf(z, 0) || math.IsNaN(z) {
		t.Errorf("ZScore must not be Inf or NaN when mean=0 stddev=0, got %f", z)
	}
	if z <= 0 {
		t.Errorf("expected positive z-score for value=10 mean=0 stddev=0, got %f", z)
	}
}

func TestZScore_ConstantInputsReturnZero(t *testing.T) {
	z := ZScore(5.0, 5.0, 0.0)
	if z != 0 {
		t.Errorf("expected z=0 for value=mean=5, got %f", z)
	}
}

func TestCompositeAnomalyScore_KnownInputs(t *testing.T) {
	score := CompositeAnomalyScore(3.0, 2.0, 0.1)
	expected := math.Abs(3.0)*0.3 + math.Abs(2.0)*0.3 + 0.1*5*0.4
	if math.Abs(score-expected) > 1e-9 {
		t.Errorf("composite score %f != expected %f", score, expected)
	}
}

func TestCompositeAnomalyScore_AllZero(t *testing.T) {
	score := CompositeAnomalyScore(0, 0, 0)
	if score != 0 {
		t.Errorf("expected 0 for all-zero inputs, got %f", score)
	}
}
