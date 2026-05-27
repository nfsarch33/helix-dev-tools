package fleeteval

import (
	"regexp"
	"strings"
)

var (
	thinkBlockRE  = regexp.MustCompile(`(?s)<think>.*?</think>`)
	thinkUnclosed = regexp.MustCompile(`(?s)<think>.*$`)
)

// CleanResponse removes model artifacts like <think> blocks, task_claim/complete
// protocol wrapping, and leading/trailing whitespace.
func CleanResponse(raw string) string {
	cleaned := thinkBlockRE.ReplaceAllString(raw, "")
	if strings.Contains(cleaned, "<think>") {
		cleaned = thinkUnclosed.ReplaceAllString(cleaned, "")
	}

	lines := strings.Split(cleaned, "\n")
	var filtered []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "task_claim(") ||
			strings.HasPrefix(trimmed, "task_complete(") ||
			strings.HasPrefix(trimmed, "handoff_publish(") ||
			strings.HasPrefix(trimmed, "agent_register(") ||
			strings.HasPrefix(trimmed, "sprint_status(") {
			continue
		}
		if trimmed == "```go" || trimmed == "```json" || trimmed == "```" {
			continue
		}
		filtered = append(filtered, line)
	}
	result := strings.Join(filtered, "\n")
	return strings.TrimSpace(result)
}

// GradeResponse scores an LLM response against a task's criteria.
// Returns a score (0 to task.Grading.MaxScore) and detail entries.
func GradeResponse(task Task, response string) (int, []GradeEntry, error) {
	maxScore := task.Grading.MaxScore
	if maxScore == 0 {
		maxScore = 10
	}

	matched, err := matchPattern(task.ExpectedPattern, response)
	if err != nil {
		return 0, nil, err
	}

	if !matched {
		return 0, []GradeEntry{{
			Metric: "pattern_match",
			Score:  0,
			Weight: 1.0,
			Note:   "response did not match expected pattern",
		}}, nil
	}

	if len(task.Grading.QualityRubric) == 0 {
		return maxScore, []GradeEntry{{
			Metric: "pattern_match",
			Score:  1.0,
			Weight: 1.0,
			Note:   "pattern matched, no rubric defined",
		}}, nil
	}

	var entries []GradeEntry
	totalWeighted := 0.0
	totalWeight := 0.0

	for _, rubric := range task.Grading.QualityRubric {
		score := scoreRubricEntry(rubric, response)
		entries = append(entries, GradeEntry{
			Metric: rubric.Metric,
			Score:  score,
			Weight: rubric.Weight,
		})
		totalWeighted += score * rubric.Weight
		totalWeight += rubric.Weight
	}

	if totalWeight == 0 {
		totalWeight = 1
	}
	normalized := totalWeighted / totalWeight
	finalScore := int(normalized * float64(maxScore))

	if finalScore > maxScore {
		finalScore = maxScore
	}
	return finalScore, entries, nil
}

// MatchesPattern checks if a response matches the expected regex pattern.
func MatchesPattern(pattern, response string) (bool, error) {
	return matchPattern(pattern, response)
}

func matchPattern(pattern, response string) (bool, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, err
	}
	return re.MatchString(response), nil
}

func scoreRubricEntry(rubric RubricEntry, response string) float64 {
	r := strings.ToLower(response)
	m := strings.ToLower(rubric.Metric)

	switch m {
	case "correctness":
		if len(response) > 10 {
			return 0.8
		}
		return 0.4
	case "tool_usage":
		if containsAny(r, []string{"tool", "command", "run", "execute", "shell", "wc", "ls", "cat", "grep"}) {
			return 0.9
		}
		return 0.3
	case "completeness":
		if len(response) > 50 {
			return 0.8
		}
		return 0.4
	case "accuracy":
		if len(response) > 20 {
			return 0.7
		}
		return 0.3
	case "formatting":
		if strings.Contains(response, "\n") || len(response) > 30 {
			return 0.7
		}
		return 0.4
	case "conciseness":
		if len(response) < 500 && len(response) > 20 {
			return 0.8
		}
		return 0.5
	case "timeliness":
		return 0.7
	case "comprehension":
		if containsAny(r, []string{"mcp", "server", "tool", "sprintboard"}) {
			return 0.8
		}
		return 0.4
	case "coverage":
		if strings.Count(response, "\n") >= 2 {
			return 0.7
		}
		return 0.4
	case "style":
		if strings.Contains(r, "t.run") || strings.Contains(r, "func test") {
			return 0.8
		}
		return 0.4
	case "execution":
		if containsAny(r, []string{"error", "output", "result", "ran", "command"}) {
			return 0.7
		}
		return 0.3
	case "diagnosis":
		if containsAny(r, []string{"because", "means", "reason", "package not found", "cannot find",
			"integer division", "truncat", "before conversion", "division by zero", "bug"}) {
			return 0.8
		}
		return 0.3
	case "remediation":
		if containsAny(r, []string{"fix", "correct", "use", "instead", "change", "replace"}) {
			return 0.8
		}
		return 0.3
	case "context":
		return 0.6
	default:
		if len(response) > 20 {
			return 0.6
		}
		return 0.3
	}
}

func containsAny(s string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}
