package eval

import "testing"

func TestComputeSessionQuality_HighQuality(t *testing.T) {
	results := []EvalResult{
		{Pass: true, Score: 1.0},
		{Pass: true, Score: 0.9},
	}

	q := ComputeSessionQuality("s1", "cursor-parent", results, 8, 2, 5, 20)

	if q.CompositeScore < 0.85 {
		t.Errorf("composite = %f, want >= 0.85 for high-quality session", q.CompositeScore)
	}
	if q.Badge != "Platinum" && q.Badge != "Gold" {
		t.Errorf("badge = %q, want Platinum or Gold", q.Badge)
	}
}

func TestComputeSessionQuality_LowQuality(t *testing.T) {
	results := []EvalResult{
		{Pass: false, Score: 0.2},
	}

	q := ComputeSessionQuality("s2", "codex", results, 1, 9, 1, 2)

	if q.CompositeScore > 0.3 {
		t.Errorf("composite = %f, want <= 0.3 for low-quality session", q.CompositeScore)
	}
	if q.Badge != "None" {
		t.Errorf("badge = %q, want None", q.Badge)
	}
}

func TestComputeSessionQuality_NoEvals(t *testing.T) {
	q := ComputeSessionQuality("s3", "agent", nil, 5, 0, 3, 10)

	if q.EvalScore != 0 {
		t.Errorf("eval_score = %f, want 0 (no evals)", q.EvalScore)
	}
	if q.SignalScore != 1.0 {
		t.Errorf("signal_score = %f, want 1.0 (5/5 accepts)", q.SignalScore)
	}
}

func TestComputeSessionQuality_NoSignals(t *testing.T) {
	results := []EvalResult{{Pass: true, Score: 1.0}}
	q := ComputeSessionQuality("s4", "agent", results, 0, 0, 5, 20)

	if q.SignalScore != 0 {
		t.Errorf("signal_score = %f, want 0 (no signals)", q.SignalScore)
	}
}

func TestComputeTrend_Improving(t *testing.T) {
	sessions := []SessionQuality{
		{CompositeScore: 0.5},
		{CompositeScore: 0.8},
	}

	trend := ComputeTrend(sessions)
	if trend.Direction != "improving" {
		t.Errorf("direction = %q, want improving", trend.Direction)
	}
	if trend.Delta < 0.25 {
		t.Errorf("delta = %f, want >= 0.25", trend.Delta)
	}
}

func TestComputeTrend_Declining(t *testing.T) {
	sessions := []SessionQuality{
		{CompositeScore: 0.9},
		{CompositeScore: 0.4},
	}

	trend := ComputeTrend(sessions)
	if trend.Direction != "declining" {
		t.Errorf("direction = %q, want declining", trend.Direction)
	}
}

func TestComputeTrend_Stable(t *testing.T) {
	sessions := []SessionQuality{
		{CompositeScore: 0.7},
		{CompositeScore: 0.72},
	}

	trend := ComputeTrend(sessions)
	if trend.Direction != "stable" {
		t.Errorf("direction = %q, want stable (delta < 0.05)", trend.Direction)
	}
}

func TestComputeTrend_InsufficientData(t *testing.T) {
	trend := ComputeTrend([]SessionQuality{{CompositeScore: 0.5}})
	if trend.Direction != "insufficient_data" {
		t.Errorf("direction = %q, want insufficient_data", trend.Direction)
	}
}
