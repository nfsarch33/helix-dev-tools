package eval

import (
	"time"
)

type Runner struct {
	MaxIterations int
}

func NewRunner() *Runner {
	return &Runner{MaxIterations: 3}
}

func (r *Runner) Run(def EvalDef) EvalResult {
	start := time.Now()

	if err := def.Validate(); err != nil {
		return EvalResult{
			EvalID:    def.ID,
			EvalName:  def.Name,
			EvalType:  def.Type,
			Pass:      false,
			Timestamp: start,
			Error:     err.Error(),
		}
	}

	maxIter := def.MaxIterations
	if maxIter <= 0 {
		maxIter = r.MaxIterations
	}

	var bestResult EvalResult

	for iter := 1; iter <= maxIter; iter++ {
		result := r.runOnce(def, iter)
		result.DurationMS = time.Since(start).Milliseconds()

		if result.Pass {
			return result
		}
		bestResult = result
	}

	return bestResult
}

func (r *Runner) runOnce(def EvalDef, iteration int) EvalResult {
	results := make([]CriterionResult, 0, len(def.Criteria))
	totalScore := 0.0
	totalWeight := 0.0
	allPass := true

	for _, criterion := range def.Criteria {
		grader := NewGrader(criterion)
		score, details, err := grader.Grade(def.Task)

		weight := criterion.Weight
		if weight <= 0 {
			weight = 1.0
		}

		cr := CriterionResult{
			Name:    criterion.Name,
			Pass:    score >= 0.5,
			Score:   score,
			Details: details,
		}

		if err != nil {
			cr.Details = err.Error()
			cr.Pass = false
			cr.Score = 0
		}

		if !cr.Pass {
			allPass = false
		}

		totalScore += score * weight
		totalWeight += weight
		results = append(results, cr)
	}

	compositeScore := 0.0
	if totalWeight > 0 {
		compositeScore = totalScore / totalWeight
	}

	return EvalResult{
		EvalID:     def.ID,
		EvalName:   def.Name,
		EvalType:   def.Type,
		Pass:       allPass,
		Score:      compositeScore,
		Criteria:   results,
		Iterations: iteration,
		Timestamp:  time.Now(),
	}
}
