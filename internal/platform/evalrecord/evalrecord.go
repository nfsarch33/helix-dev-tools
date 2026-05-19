package evalrecord

import (
	"encoding/json"
	"fmt"
	"time"
)

type Verdict string

const (
	VerdictPass Verdict = "pass"
	VerdictFail Verdict = "fail"
	VerdictSkip Verdict = "skip"
)

type Record struct {
	ID            string    `json:"id"`
	Branch        string    `json:"branch"`
	TicketID      string    `json:"ticket_id"`
	AgentID       string    `json:"agent_id"`
	TestsRun      int       `json:"tests_run"`
	TestsPassed   int       `json:"tests_passed"`
	SentruxBefore int       `json:"sentrux_before"`
	SentruxAfter  int       `json:"sentrux_after"`
	MCPHealthy    bool      `json:"mcp_healthy"`
	ElapsedSec    float64   `json:"elapsed_sec"`
	Verdict       Verdict   `json:"verdict"`
	RiskScore     int       `json:"risk_score"`
	EvidenceLink  string    `json:"evidence_link"`
	RecordedAt    time.Time `json:"recorded_at"`
}

func New(branch, ticketID, agentID string) *Record {
	return &Record{
		ID:         fmt.Sprintf("eval-%s-%d", ticketID, time.Now().UnixMilli()),
		Branch:     branch,
		TicketID:   ticketID,
		AgentID:    agentID,
		RecordedAt: time.Now(),
	}
}

func (r *Record) SetTests(run, passed int) {
	r.TestsRun = run
	r.TestsPassed = passed
}

func (r *Record) SetSentrux(before, after int) {
	r.SentruxBefore = before
	r.SentruxAfter = after
}

func (r *Record) SetMCPHealth(healthy bool) {
	r.MCPHealthy = healthy
}

func (r *Record) SetElapsed(d time.Duration) {
	r.ElapsedSec = d.Seconds()
}

func (r *Record) SetEvidence(link string) {
	r.EvidenceLink = link
}

func (r *Record) Evaluate() {
	r.RiskScore = 0
	if r.TestsRun > 0 && r.TestsPassed < r.TestsRun {
		r.RiskScore += 50
	}
	if r.SentruxAfter > 0 && r.SentruxAfter < r.SentruxBefore {
		r.RiskScore += 30
	}
	if !r.MCPHealthy {
		r.RiskScore += 20
	}

	if r.RiskScore == 0 && r.TestsRun > 0 {
		r.Verdict = VerdictPass
	} else if r.TestsRun == 0 {
		r.Verdict = VerdictSkip
	} else {
		r.Verdict = VerdictFail
	}
}

func (r *Record) JSON() string {
	data, _ := json.Marshal(r)
	return string(data)
}

func (r *Record) IsEvoSpineCandidate() bool {
	return r.Verdict == VerdictPass && r.SentruxAfter > r.SentruxBefore
}
