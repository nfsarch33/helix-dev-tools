package eval

// ReflectConfig controls the ReflectAndRefine loop.
type ReflectConfig struct {
	// MaxIterations is the maximum number of times to run the eval.
	// Defaults to 3 when zero.
	MaxIterations int
	// ScoreThreshold is the score at which iteration stops early (>= comparison).
	// Defaults to 0.9 when zero.
	ScoreThreshold float64
}

func (c *ReflectConfig) applyDefaults() {
	if c.MaxIterations <= 0 {
		c.MaxIterations = 3
	}
	if c.ScoreThreshold <= 0 {
		c.ScoreThreshold = 0.9
	}
}

// ReflectResult holds the outcome of a ReflectAndRefine run.
type ReflectResult struct {
	// Iterations is the number of times the eval was run.
	Iterations int
	// FinalScore is the best score seen across all iterations.
	FinalScore float64
	// Improved is true when the final score is strictly greater than the
	// score of the first iteration.
	Improved bool
	// History contains every EvalResult in run order.
	History []EvalResult
}

// ReflectAndRefine runs def repeatedly, stopping as soon as the score meets
// cfg.ScoreThreshold or cfg.MaxIterations is reached.
// Each iteration is independent (same EvalDef, same Runner).
func ReflectAndRefine(def EvalDef, cfg ReflectConfig) ReflectResult {
	cfg.applyDefaults()

	var history []EvalResult
	bestScore := -1.0
	firstScore := 0.0

	for i := 0; i < cfg.MaxIterations; i++ {
		result := NewRunner().Run(def)
		history = append(history, result)

		if i == 0 {
			firstScore = result.Score
		}

		if result.Score > bestScore {
			bestScore = result.Score
		}

		if result.Score >= cfg.ScoreThreshold {
			break
		}
	}

	if bestScore < 0 {
		bestScore = 0
	}

	return ReflectResult{
		Iterations: len(history),
		FinalScore: bestScore,
		Improved:   bestScore > firstScore,
		History:    history,
	}
}
