package workspace

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
	"time"
)

type CoverageInput struct {
	WorkspaceEvents io.Reader
	MetricsEvents   io.Reader
	Since           time.Time
	Now             time.Time
}

type CoverageSummary struct {
	Since             time.Time `json:"since"`
	Now               time.Time `json:"now"`
	WorkspaceRuns     int       `json:"workspace_runs"`
	GreenCount        int       `json:"green_count"`
	YellowCount       int       `json:"yellow_count"`
	RedCount          int       `json:"red_count"`
	GitMutationEvents int       `json:"git_mutation_events"`
	PostShellEvents   int       `json:"post_shell_events"`
	HookHitRate       float64   `json:"hook_hit_rate"`
}

func SummariseCoverage(input CoverageInput) (CoverageSummary, error) {
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	summary := CoverageSummary{Since: input.Since, Now: now}
	if err := scanWorkspaceEvents(input.WorkspaceEvents, input.Since, &summary); err != nil {
		return CoverageSummary{}, err
	}
	if err := scanMetricEvents(input.MetricsEvents, input.Since, &summary); err != nil {
		return CoverageSummary{}, err
	}
	if summary.GitMutationEvents > 0 {
		summary.HookHitRate = float64(summary.PostShellEvents) / float64(summary.GitMutationEvents) * 100
	}
	return summary, nil
}

func scanWorkspaceEvents(r io.Reader, since time.Time, summary *CoverageSummary) error {
	if r == nil {
		return nil
	}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		var event struct {
			GeneratedAt time.Time `json:"generated_at"`
			Timestamp   time.Time `json:"ts"`
			Tier        string    `json:"tier"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue
		}
		at := event.GeneratedAt
		if at.IsZero() {
			at = event.Timestamp
		}
		if !at.IsZero() && at.Before(since) {
			continue
		}
		summary.WorkspaceRuns++
		switch strings.ToUpper(event.Tier) {
		case "RED":
			summary.RedCount++
		case "YELLOW":
			summary.YellowCount++
		default:
			summary.GreenCount++
		}
	}
	return scanner.Err()
}

func scanMetricEvents(r io.Reader, since time.Time, summary *CoverageSummary) error {
	if r == nil {
		return nil
	}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		var event struct {
			Timestamp time.Time `json:"ts"`
			Hook      string    `json:"hook"`
			Detail    string    `json:"detail"`
			Category  string    `json:"cat"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue
		}
		if !event.Timestamp.IsZero() && event.Timestamp.Before(since) {
			continue
		}
		if event.Hook == "post-shell" {
			summary.PostShellEvents++
			continue
		}
		if event.Hook == "guard-shell" && looksLikeGitMutation(event.Detail) {
			summary.GitMutationEvents++
		}
	}
	return scanner.Err()
}

func looksLikeGitMutation(detail string) bool {
	detail = strings.ToLower(detail)
	if !(strings.Contains(detail, "git ") || strings.Contains(detail, "runx git") || strings.Contains(detail, "runx pr") || strings.Contains(detail, "gh ")) {
		return false
	}
	for _, word := range []string{"commit", "push", "merge", "rebase", "cherry-pick", "revert"} {
		if strings.Contains(detail, word) {
			return true
		}
	}
	return false
}
