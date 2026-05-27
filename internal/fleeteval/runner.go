package fleeteval

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nfsarch33/helix-dev-tools/internal/fleetagent"
)

// RunnerConfig configures the eval runner.
type RunnerConfig struct {
	TaskFilePath   string
	SystemPrompt   string
	Model          string
	TimeoutSeconds int
}

// Runner executes eval tasks against an LLM and grades responses.
type Runner struct {
	llm fleetagent.LLMClient
	cfg RunnerConfig
	log *slog.Logger
}

// NewRunner creates an eval runner.
func NewRunner(llm fleetagent.LLMClient, cfg RunnerConfig, log *slog.Logger) *Runner {
	if log == nil {
		log = slog.Default()
	}
	return &Runner{llm: llm, cfg: cfg, log: log}
}

// RunAll executes all tasks from the task file and returns a report.
func (r *Runner) RunAll(ctx context.Context, tasks []Task) (*RunReport, error) {
	runID := fmt.Sprintf("fleet-eval-%s", time.Now().Format("20060102-150405"))
	report := &RunReport{
		RunID:     runID,
		Model:     r.cfg.Model,
		Timestamp: time.Now(),
	}

	var totalDuration int64
	for _, task := range tasks {
		result := r.runTask(ctx, task)
		report.Results = append(report.Results, result)
		report.TotalScore += result.Score
		report.MaxScore += result.MaxScore
		totalDuration += result.DurationMS

		if result.Error != "" {
			report.ErrorCount++
		} else if result.Pass {
			report.PassCount++
		} else {
			report.FailCount++
		}
	}

	report.TotalTasks = len(tasks)
	if report.TotalTasks > 0 {
		report.PassRate = float64(report.PassCount) / float64(report.TotalTasks)
		report.AvgDurationMS = totalDuration / int64(report.TotalTasks)
	}

	report.Verdict = verdictFromScore(report.TotalScore, report.MaxScore)
	return report, nil
}

func (r *Runner) runTask(ctx context.Context, task Task) Result {
	start := time.Now()
	timeout := time.Duration(r.cfg.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 120 * time.Second
	}

	taskCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	r.log.Info("running eval task", "task_id", task.ID, "level", task.Level, "title", task.Title)

	response, err := r.llm.Complete(taskCtx, r.cfg.SystemPrompt, buildEvalPrompt(task))
	duration := time.Since(start)

	if err != nil {
		return Result{
			TaskID:     task.ID,
			Level:      task.Level,
			Title:      task.Title,
			Score:      0,
			MaxScore:   task.Grading.MaxScore,
			Pass:       false,
			Response:   "",
			Error:      err.Error(),
			DurationMS: duration.Milliseconds(),
			Timestamp:  start,
		}
	}

	score, details, gradeErr := GradeResponse(task, response)
	matched, _ := MatchesPattern(task.ExpectedPattern, response)

	maxScore := task.Grading.MaxScore
	if maxScore == 0 {
		maxScore = 10
	}
	threshold := task.Grading.PassThreshold
	if threshold == 0 {
		threshold = maxScore / 2
	}

	result := Result{
		TaskID:       task.ID,
		Level:        task.Level,
		Title:        task.Title,
		Score:        score,
		MaxScore:     maxScore,
		Pass:         score >= threshold,
		PatternMatch: matched,
		Response:     truncate(response, 2000),
		DurationMS:   duration.Milliseconds(),
		Timestamp:    start,
		GradeDetail:  details,
	}
	if gradeErr != nil {
		result.Error = gradeErr.Error()
	}
	return result
}

func buildEvalPrompt(task Task) string {
	return fmt.Sprintf("Task ID: %s\nLevel: %d\nTitle: %s\n\n%s\n\nRespond with your answer only. Be concise.",
		task.ID, task.Level, task.Title, task.Description)
}

func verdictFromScore(total, max int) string {
	if max == 0 {
		return "no_tasks"
	}
	pct := float64(total) / float64(max) * 100
	switch {
	case pct >= 80:
		return "GREEN"
	case pct >= 50:
		return "YELLOW"
	default:
		return "RED"
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...[truncated]"
}
