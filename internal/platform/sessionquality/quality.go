package sessionquality

import "math"

// Badge represents a quality tier.
type Badge string

const (
	BadgeBronze   Badge = "Bronze"
	BadgeSilver   Badge = "Silver"
	BadgeGold     Badge = "Gold"
	BadgePlatinum Badge = "Platinum"
)

// SessionScore captures quality metrics for a single session.
type SessionScore struct {
	SessionID  string
	EvalScore  float64 // 0.0-1.0 from eval results
	SignalRate float64 // accept/(accept+reject) from signal data
	Score      float64 // computed composite 0.0-1.0
	Badge      Badge
}

// Compute calculates the composite score and badge.
// evalScore: fraction of evals that passed (0.0-1.0)
// acceptSignals: count of accept signals in the session
// rejectSignals: count of reject signals in the session
func Compute(sessionID string, evalScore float64, acceptSignals, rejectSignals int) SessionScore {
	signalRate := 0.5
	total := acceptSignals + rejectSignals
	if total > 0 {
		signalRate = float64(acceptSignals) / float64(total)
	}

	score := 0.6*evalScore + 0.4*signalRate
	score = math.Round(score*1000) / 1000

	return SessionScore{
		SessionID:  sessionID,
		EvalScore:  evalScore,
		SignalRate: signalRate,
		Score:      score,
		Badge:      badgeFor(score),
	}
}

func badgeFor(score float64) Badge {
	switch {
	case score >= 0.90:
		return BadgePlatinum
	case score >= 0.75:
		return BadgeGold
	case score >= 0.60:
		return BadgeSilver
	default:
		return BadgeBronze
	}
}

// Trend computes the quality trend over the last N sessions.
// Returns positive value if improving, negative if declining.
func Trend(scores []SessionScore) float64 {
	if len(scores) < 2 {
		return 0
	}
	n := len(scores)
	return scores[n-1].Score - scores[0].Score
}

// RegressionAlert returns true if the latest score regressed vs the rolling average.
func RegressionAlert(scores []SessionScore) bool {
	if len(scores) < 2 {
		return false
	}
	var sum float64
	baseline := scores[:len(scores)-1]
	for _, s := range baseline {
		sum += s.Score
	}
	avg := sum / float64(len(baseline))
	latest := scores[len(scores)-1].Score
	return latest < avg-0.05
}
