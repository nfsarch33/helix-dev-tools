package eval

func PassAtK(results []EvalResult, k int) float64 {
	if k <= 0 || len(results) == 0 {
		return 0
	}

	limit := k
	if limit > len(results) {
		limit = len(results)
	}

	for i := 0; i < limit; i++ {
		if results[i].Pass {
			return 1.0
		}
	}
	return 0
}

func PassPowerK(results []EvalResult, k int) float64 {
	if k <= 0 || len(results) == 0 {
		return 0
	}

	limit := k
	if limit > len(results) {
		limit = len(results)
	}

	for i := 0; i < limit; i++ {
		if !results[i].Pass {
			return 0
		}
	}
	return 1.0
}

func PassRate(results []EvalResult) float64 {
	if len(results) == 0 {
		return 0
	}
	passed := 0
	for _, r := range results {
		if r.Pass {
			passed++
		}
	}
	return float64(passed) / float64(len(results))
}

func AverageScore(results []EvalResult) float64 {
	if len(results) == 0 {
		return 0
	}
	total := 0.0
	for _, r := range results {
		total += r.Score
	}
	return total / float64(len(results))
}
