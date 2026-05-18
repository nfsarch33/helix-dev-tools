package sessionquality

import "testing"

func TestCompute_PlatinumBadge(t *testing.T) {
	s := Compute("s1", 1.0, 10, 0)
	if s.Badge != BadgePlatinum {
		t.Errorf("expected Platinum, got %s (score=%.3f)", s.Badge, s.Score)
	}
}

func TestCompute_GoldBadge(t *testing.T) {
	s := Compute("s1", 0.8, 7, 3)
	if s.Badge != BadgeGold {
		t.Errorf("expected Gold, got %s (score=%.3f)", s.Badge, s.Score)
	}
}

func TestCompute_SilverBadge(t *testing.T) {
	// score = 0.6*0.7 + 0.4*0.5 = 0.42+0.20 = 0.62 -> Silver
	s := Compute("s1", 0.7, 5, 5)
	if s.Badge != BadgeSilver {
		t.Errorf("expected Silver, got %s (score=%.3f)", s.Badge, s.Score)
	}
}

func TestCompute_BronzeBadge(t *testing.T) {
	s := Compute("s1", 0.0, 0, 10)
	if s.Badge != BadgeBronze {
		t.Errorf("expected Bronze, got %s (score=%.3f)", s.Badge, s.Score)
	}
}

func TestCompute_NoSignals_DefaultRate(t *testing.T) {
	s := Compute("s1", 1.0, 0, 0)
	// signalRate defaults to 0.5, so score = 0.6*1.0 + 0.4*0.5 = 0.8
	if s.Score < 0.79 || s.Score > 0.81 {
		t.Errorf("expected score ~0.8, got %.3f", s.Score)
	}
}

func TestTrend_Improving(t *testing.T) {
	scores := []SessionScore{
		{Score: 0.5},
		{Score: 0.6},
		{Score: 0.7},
	}
	if Trend(scores) <= 0 {
		t.Error("expected positive trend")
	}
}

func TestTrend_Declining(t *testing.T) {
	scores := []SessionScore{
		{Score: 0.8},
		{Score: 0.7},
		{Score: 0.6},
	}
	if Trend(scores) >= 0 {
		t.Error("expected negative trend")
	}
}

func TestTrend_SingleSession(t *testing.T) {
	if Trend([]SessionScore{{Score: 0.9}}) != 0 {
		t.Error("single session trend should be 0")
	}
}

func TestRegressionAlert_Triggered(t *testing.T) {
	scores := []SessionScore{
		{Score: 0.9},
		{Score: 0.9},
		{Score: 0.9},
		{Score: 0.5}, // big drop
	}
	if !RegressionAlert(scores) {
		t.Error("expected regression alert")
	}
}

func TestRegressionAlert_NotTriggered(t *testing.T) {
	scores := []SessionScore{
		{Score: 0.8},
		{Score: 0.85},
		{Score: 0.82},
	}
	if RegressionAlert(scores) {
		t.Error("expected no regression alert for stable scores")
	}
}
