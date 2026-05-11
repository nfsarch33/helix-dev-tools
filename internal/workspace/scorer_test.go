package workspace

import "testing"

func TestScoreReport_TierBoundaries(t *testing.T) {
	tests := []struct {
		name string
		in   AuditReport
		want Tier
	}{
		{
			name: "green",
			in:   AuditReport{Repos: []RepoStatus{{Alias: "global-kb"}}},
			want: TierGreen,
		},
		{
			name: "yellow",
			in: AuditReport{Repos: []RepoStatus{{
				Alias: "business",
				Findings: []Finding{
					{Code: FindingDirtyWorktree, Severity: SeverityHard},
					{Code: FindingUnpushedCommits, Severity: SeverityWarning},
				},
			}}},
			want: TierYellow,
		},
		{
			name: "red",
			in: AuditReport{Repos: []RepoStatus{
				{Alias: "a", Findings: []Finding{{Code: FindingDirtyWorktree, Severity: SeverityHard}}},
				{Alias: "b", Findings: []Finding{{Code: FindingDirtyWorktree, Severity: SeverityHard}}},
			}},
			want: TierRed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ScoreReport(tt.in)
			if got.Tier != tt.want {
				t.Fatalf("tier = %s, want %s; score=%d", got.Tier, tt.want, got.Score)
			}
		})
	}
}

func TestScoreReport_WeightsFindings(t *testing.T) {
	got := ScoreReport(AuditReport{Repos: []RepoStatus{{
		Alias: "repo",
		Findings: []Finding{
			{Code: FindingDirtyWorktree, Severity: SeverityHard},
			{Code: FindingUnpushedCommits, Severity: SeverityWarning},
			{Code: FindingBehindDefault, Severity: SeverityInfo},
			{Code: FindingVendorBehind, Severity: SeverityInfo},
		},
	}}})

	if got.Score != 59 {
		t.Fatalf("score = %d, want 59", got.Score)
	}
	if got.Tier != TierRed {
		t.Fatalf("tier = %s, want %s", got.Tier, TierRed)
	}
}

// TestScorer_StableUnderVendorMirrorLag pins the v322-5 contract:
// when the residual finding mix is composed entirely of race-protected
// signals (vendor-mirror-behind on upstream OSS forks + dirty
// worktrees in hands-off agent territory like the EC-agent paths),
// the workspace score MUST stay in YELLOW (60-79). RED was the v321
// close-state symptom: 6 vendor-mirror-behind + 2 EC-agent dirty
// produced score 44 (RED) under the old formula, even though every
// finding was outside the worker's authority to remediate.
//
// The fix introduces FindingDirtyRaceProtected (weight 8) so that
// dirty worktrees in race-protected repos count materially less than
// agent-introduced dirty state (FindingDirtyWorktree, weight 25).
// vendor_behind already weights 1 and so naturally lands in race-
// protected territory.
func TestScorer_StableUnderVendorMirrorLag(t *testing.T) {
	report := AuditReport{Repos: []RepoStatus{
		// 6 vendor-mirror-behind findings spread across the canonical
		// vendor-mirror set (ironclaw, openclaw, hermes, gstack,
		// temporal, windows-mcp).
		{Alias: "ironclaw", Findings: []Finding{{Code: FindingVendorBehind, Severity: SeverityInfo}}},
		{Alias: "openclaw", Findings: []Finding{{Code: FindingVendorBehind, Severity: SeverityInfo}}},
		{Alias: "hermes", Findings: []Finding{{Code: FindingVendorBehind, Severity: SeverityInfo}}},
		{Alias: "gstack", Findings: []Finding{{Code: FindingVendorBehind, Severity: SeverityInfo}}},
		{Alias: "temporal", Findings: []Finding{{Code: FindingVendorBehind, Severity: SeverityInfo}}},
		{Alias: "windows-mcp", Findings: []Finding{{Code: FindingVendorBehind, Severity: SeverityInfo}}},
		// 2 EC-agent dirty worktrees: this is the v321 close-state
		// shape that incorrectly pushed the workspace into RED.
		{Alias: "agentic-ecommerce-web", Findings: []Finding{{Code: FindingDirtyRaceProtected, Severity: SeverityWarning}}},
		{Alias: "business", Findings: []Finding{{Code: FindingDirtyRaceProtected, Severity: SeverityWarning}}},
	}}

	got := ScoreReport(report)

	if got.Tier != TierYellow {
		t.Fatalf("tier = %s, want %s; score=%d (race-protected mix must not RED)", got.Tier, TierYellow, got.Score)
	}
	if got.Score < 60 || got.Score >= 80 {
		t.Fatalf("score = %d, want YELLOW range [60, 80)", got.Score)
	}
}

// TestScorer_DirtyRaceProtectedWeightsLessThanAgentDirty pins the
// weight discount: a single agent-introduced dirty worktree (weight
// 25) MUST count more than a single race-protected dirty worktree
// (weight 8). The discount is what keeps the workspace score honest
// when the only outstanding signals are out of the agent's control.
func TestScorer_DirtyRaceProtectedWeightsLessThanAgentDirty(t *testing.T) {
	agentDirty := ScoreReport(AuditReport{Repos: []RepoStatus{{
		Alias:    "agent-repo",
		Findings: []Finding{{Code: FindingDirtyWorktree, Severity: SeverityHard}},
	}}})
	raceProtected := ScoreReport(AuditReport{Repos: []RepoStatus{{
		Alias:    "ec-agent-repo",
		Findings: []Finding{{Code: FindingDirtyRaceProtected, Severity: SeverityWarning}},
	}}})

	if agentDirty.Score >= raceProtected.Score {
		t.Fatalf("agent-introduced dirty score (%d) must be lower than race-protected dirty score (%d)",
			agentDirty.Score, raceProtected.Score)
	}
	// Sanity-check the weights we picked: agent-introduced is 25 off
	// (score 75), race-protected is 8 off (score 92). If anyone
	// shifts these in the future, this guard makes the intent
	// explicit.
	if agentDirty.Score != 75 {
		t.Fatalf("agent dirty score = %d, want 75 (weight 25)", agentDirty.Score)
	}
	if raceProtected.Score != 92 {
		t.Fatalf("race-protected dirty score = %d, want 92 (weight 8)", raceProtected.Score)
	}
}
