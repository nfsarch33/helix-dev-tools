package domain

import "time"

// CycleState is the finite state machine for a review cycle.
type CycleState int

const (
	CycleIdle CycleState = iota
	CycleScanning
	CycleReporting
	CycleDone
	CycleEscalated
	CycleSkipped
)

var cycleStateNames = [...]string{
	CycleIdle:      "idle",
	CycleScanning:  "scanning",
	CycleReporting: "reporting",
	CycleDone:      "done",
	CycleEscalated: "escalated",
	CycleSkipped:   "skipped",
}

func (s CycleState) String() string {
	if int(s) < len(cycleStateNames) {
		return cycleStateNames[s]
	}
	return "unknown"
}

const escalationThresholdLOC = 300

// Cycle tracks one scan-report pass for a repository.
type Cycle struct {
	RepoAlias string     `json:"repo_alias"`
	State     CycleState `json:"state"`
	HeadSHA   string     `json:"head_sha"`
	BaseSHA   string     `json:"base_sha,omitempty"`
	LOCDelta  int        `json:"loc_delta"`
	Findings  []Finding  `json:"findings,omitempty"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   time.Time  `json:"ended_at,omitempty"`
}

// HasNewCommits returns true when head differs from base.
func (c *Cycle) HasNewCommits() bool {
	return c.HeadSHA != "" && c.HeadSHA != c.BaseSHA
}

// NeedsEscalation returns true when the LOC delta exceeds the
// threshold and the cycle should pause for operator review.
func (c *Cycle) NeedsEscalation() bool {
	return c.LOCDelta >= escalationThresholdLOC
}
