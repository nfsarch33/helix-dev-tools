package eval

import "time"

type SessionQuality struct {
	SessionID    string    `json:"session_id"`
	AgentID      string    `json:"agent_id"`
	EvalScore    float64   `json:"eval_score"`
	SignalScore  float64   `json:"signal_score"`
	CompositeScore float64 `json:"composite_score"`
	Badge        string    `json:"badge"`
	StoriesCompleted int   `json:"stories_completed"`
	TestsPassed  int       `json:"tests_passed"`
	Timestamp    time.Time `json:"timestamp"`
}

type QualityTrend struct {
	Direction   string  `json:"direction"`
	Delta       float64 `json:"delta"`
	SessionCount int    `json:"session_count"`
}

const (
	evalWeight   = 0.6
	signalWeight = 0.2
	storyWeight  = 0.1
	testWeight   = 0.1
)

func ComputeSessionQuality(sessionID, agentID string, evalResults []EvalResult, acceptCount, rejectCount, storiesCompleted, testsPassed int) SessionQuality {
	evalScore := 0.0
	if len(evalResults) > 0 {
		evalScore = PassRate(evalResults)
	}

	signalScore := 0.0
	total := acceptCount + rejectCount
	if total > 0 {
		signalScore = float64(acceptCount) / float64(total)
	}

	storyScore := 0.0
	if storiesCompleted >= 5 {
		storyScore = 1.0
	} else if storiesCompleted > 0 {
		storyScore = float64(storiesCompleted) / 5.0
	}

	testScore := 0.0
	if testsPassed >= 20 {
		testScore = 1.0
	} else if testsPassed > 0 {
		testScore = float64(testsPassed) / 20.0
	}

	composite := evalScore*evalWeight +
		signalScore*signalWeight +
		storyScore*storyWeight +
		testScore*testWeight

	return SessionQuality{
		SessionID:        sessionID,
		AgentID:          agentID,
		EvalScore:        evalScore,
		SignalScore:       signalScore,
		CompositeScore:   composite,
		Badge:            QualityBadge(composite),
		StoriesCompleted: storiesCompleted,
		TestsPassed:      testsPassed,
		Timestamp:        time.Now(),
	}
}

func ComputeTrend(sessions []SessionQuality) QualityTrend {
	if len(sessions) < 2 {
		return QualityTrend{Direction: "insufficient_data", SessionCount: len(sessions)}
	}

	recent := sessions[len(sessions)-1].CompositeScore
	previous := sessions[len(sessions)-2].CompositeScore
	delta := recent - previous

	direction := "stable"
	if delta > 0.05 {
		direction = "improving"
	} else if delta < -0.05 {
		direction = "declining"
	}

	return QualityTrend{
		Direction:    direction,
		Delta:        delta,
		SessionCount: len(sessions),
	}
}
