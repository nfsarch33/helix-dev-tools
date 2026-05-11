package workspace

import "time"

// findingWeights are tuned so that the workspace score reflects
// signal strength: an agent-introduced dirty worktree (weight 25)
// dominates the score because the agent can fix it immediately,
// while a race-protected dirty worktree (weight 8, see v322-5) and
// vendor-mirror-behind (weight 1) count materially less because the
// worker has no authority to remediate them. The v322-5 retro
// captures the formula derivation; see
// reports/research/v322-5-workspace-doctor-scoring-investigation-2026-05-09.md.
var findingWeights = map[FindingCode]int{
	FindingDirtyWorktree:      25,
	FindingDirtyRaceProtected: 8,
	FindingUnpushedCommits:    10,
	FindingDetachedHead:       15,
	FindingWrongIdentity:      5,
	FindingBehindDefault:      5,
	FindingStaleTrackingRef:   2,
	FindingVendorBehind:       1,
	FindingNoMainRef:          10,
	FindingAuditError:         10,
}

func ScoreReport(report AuditReport) Score {
	score := 100
	findings := 0
	for repoIndex := range report.Repos {
		for findingIndex := range report.Repos[repoIndex].Findings {
			finding := &report.Repos[repoIndex].Findings[findingIndex]
			weight := findingWeights[finding.Code]
			if weight == 0 {
				weight = 1
			}
			finding.Weight = weight
			score -= weight
			findings++
		}
	}
	if score < 0 {
		score = 0
	}
	generatedAt := report.GeneratedAt
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}
	return Score{
		GeneratedAt: generatedAt,
		Score:       score,
		Tier:        tierForScore(score),
		Findings:    findings,
		Repos:       report.Repos,
	}
}

func tierForScore(score int) Tier {
	switch {
	case score < 60:
		return TierRed
	case score < 80:
		return TierYellow
	default:
		return TierGreen
	}
}
