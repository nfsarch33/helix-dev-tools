package hookmetrics

import (
	"sort"
	"time"

	"github.com/nfsarch33/cursor-tools/internal/metrics"
)

// AuditResult summarizes whether hook fire coverage meets a target threshold.
type AuditResult struct {
	Mutations       int
	HookFires       int
	HitRate         float64
	Threshold       float64
	BelowThreshold  bool
	Underperforming []string
}

// Audit computes fleet hook hit rate for the requested window. A mutation is
// covered when at least one hook fire is observed for the same TurnID.
func Audit(events []metrics.Event, since time.Time, threshold float64) AuditResult {
	mutationsByTurn := map[string]int{}
	hooksByTurn := map[string]int{}
	reposMissingHooks := map[string]bool{}

	for _, event := range events {
		if event.Timestamp.Before(since) {
			continue
		}
		turn := event.TurnID
		if turn == "" {
			turn = event.Detail
		}
		if isGitMutation(event) {
			mutationsByTurn[turn]++
			if event.Profile != "" {
				reposMissingHooks[event.Profile] = true
			}
		}
		if event.Hook != "" && event.Hook != "git-mutation" {
			hooksByTurn[turn]++
			if event.Profile != "" {
				delete(reposMissingHooks, event.Profile)
			}
		}
	}

	result := AuditResult{Threshold: threshold}
	for turn, count := range mutationsByTurn {
		result.Mutations += count
		if hooksByTurn[turn] > 0 {
			result.HookFires += count
		}
	}
	if result.Mutations > 0 {
		result.HitRate = float64(result.HookFires) / float64(result.Mutations)
	}
	result.BelowThreshold = result.HitRate < threshold
	for repo := range reposMissingHooks {
		result.Underperforming = append(result.Underperforming, repo)
	}
	sort.Strings(result.Underperforming)
	return result
}
