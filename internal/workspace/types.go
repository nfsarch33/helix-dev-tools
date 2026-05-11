package workspace

import "time"

type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityHard    Severity = "hard"
)

type Tier string

const (
	TierGreen  Tier = "GREEN"
	TierYellow Tier = "YELLOW"
	TierRed    Tier = "RED"
)

type FindingCode string

const (
	FindingDirtyWorktree FindingCode = "dirty_worktree"
	// FindingDirtyRaceProtected is the v322-5 race-protected sibling
	// of FindingDirtyWorktree: the same dirty-worktree symptom but
	// in a repo where the worker has no authority to remediate (a
	// vendor mirror lagging upstream OSS, or an alias owned by the
	// hands-off EC-agent / operator territory). Race-protected
	// findings count materially less than agent-introduced findings
	// in the workspace doctor scorer (weight 8 vs 25), preventing
	// the v321 false-RED close state where 6 vendor-mirror-behind +
	// 2 EC-agent dirty produced score 44 even though every finding
	// was outside the worker's authority.
	FindingDirtyRaceProtected FindingCode = "dirty_race_protected"
	FindingUnpushedCommits    FindingCode = "unpushed_commits"
	FindingBehindDefault      FindingCode = "behind_default"
	FindingDetachedHead       FindingCode = "detached_head"
	FindingStaleTrackingRef   FindingCode = "stale_tracking_ref"
	FindingVendorBehind       FindingCode = "vendor_behind"
	FindingWrongIdentity      FindingCode = "wrong_identity"
	FindingNoMainRef          FindingCode = "no_main_ref"
	FindingAuditError         FindingCode = "audit_error"
)

type Finding struct {
	Code     FindingCode `json:"code"`
	Severity Severity    `json:"severity"`
	Weight   int         `json:"weight"`
	Message  string      `json:"message"`
}

type RepoConfig struct {
	Alias         string
	Path          string
	Identity      string
	DefaultBranch string
	VendorMirror  bool
	// RaceProtected marks repos where the worker has no authority
	// to remediate findings (operator territory, hands-off EC-agent
	// paths). Vendor mirrors are race-protected by definition; this
	// flag extends the same treatment to non-vendor-mirror aliases
	// that should not RED the workspace score when dirty.
	RaceProtected bool
}

type RepoStatus struct {
	Alias       string    `json:"alias"`
	Path        string    `json:"-"`
	Branch      string    `json:"branch,omitempty"`
	Default     string    `json:"default_branch,omitempty"`
	Ahead       int       `json:"ahead,omitempty"`
	Behind      int       `json:"behind,omitempty"`
	GeneratedAt time.Time `json:"generated_at,omitempty"`
	Findings    []Finding `json:"findings,omitempty"`
}

func (r RepoStatus) FindingCodes() []FindingCode {
	out := make([]FindingCode, 0, len(r.Findings))
	for _, finding := range r.Findings {
		out = append(out, finding.Code)
	}
	return out
}

type AuditReport struct {
	GeneratedAt time.Time    `json:"generated_at"`
	Repos       []RepoStatus `json:"repos"`
}

type Score struct {
	GeneratedAt time.Time    `json:"generated_at"`
	Score       int          `json:"score"`
	Tier        Tier         `json:"tier"`
	Findings    int          `json:"findings"`
	Repos       []RepoStatus `json:"repos"`
}
