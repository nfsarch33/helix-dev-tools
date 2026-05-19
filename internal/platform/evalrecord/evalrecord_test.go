package evalrecord_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/nfsarch33/helix-dev-tools/internal/platform/evalrecord"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	r := evalrecord.New("feat/test", "T-001", "cursor-parent")
	assert.Equal(t, "feat/test", r.Branch)
	assert.Equal(t, "T-001", r.TicketID)
	assert.Equal(t, "cursor-parent", r.AgentID)
	assert.NotEmpty(t, r.ID)
	assert.False(t, r.RecordedAt.IsZero())
}

func TestEvaluate_AllPass(t *testing.T) {
	r := evalrecord.New("feat/x", "T-1", "agent")
	r.SetTests(10, 10)
	r.SetSentrux(7000, 7020)
	r.SetMCPHealth(true)
	r.SetElapsed(5 * time.Minute)
	r.Evaluate()

	assert.Equal(t, evalrecord.VerdictPass, r.Verdict)
	assert.Equal(t, 0, r.RiskScore)
}

func TestEvaluate_TestFailure(t *testing.T) {
	r := evalrecord.New("feat/x", "T-2", "agent")
	r.SetTests(10, 8)
	r.SetSentrux(7000, 7020)
	r.SetMCPHealth(true)
	r.Evaluate()

	assert.Equal(t, evalrecord.VerdictFail, r.Verdict)
	assert.Equal(t, 50, r.RiskScore)
}

func TestEvaluate_SentruxRegression(t *testing.T) {
	r := evalrecord.New("feat/x", "T-3", "agent")
	r.SetTests(10, 10)
	r.SetSentrux(7000, 6990)
	r.SetMCPHealth(true)
	r.Evaluate()

	assert.Equal(t, evalrecord.VerdictFail, r.Verdict)
	assert.Equal(t, 30, r.RiskScore)
}

func TestEvaluate_MCPUnhealthy(t *testing.T) {
	r := evalrecord.New("feat/x", "T-4", "agent")
	r.SetTests(10, 10)
	r.SetSentrux(7000, 7000)
	r.SetMCPHealth(false)
	r.Evaluate()

	assert.Equal(t, evalrecord.VerdictFail, r.Verdict)
	assert.Equal(t, 20, r.RiskScore)
}

func TestEvaluate_NoTests(t *testing.T) {
	r := evalrecord.New("feat/x", "T-5", "agent")
	r.SetSentrux(7000, 7000)
	r.SetMCPHealth(true)
	r.Evaluate()

	assert.Equal(t, evalrecord.VerdictSkip, r.Verdict)
}

func TestJSON(t *testing.T) {
	r := evalrecord.New("feat/x", "T-6", "agent")
	r.SetTests(5, 5)
	r.SetMCPHealth(true)
	r.Evaluate()

	data := r.JSON()
	var parsed evalrecord.Record
	err := json.Unmarshal([]byte(data), &parsed)
	require.NoError(t, err)
	assert.Equal(t, "feat/x", parsed.Branch)
	assert.Equal(t, evalrecord.VerdictPass, parsed.Verdict)
}

func TestIsEvoSpineCandidate(t *testing.T) {
	r := evalrecord.New("feat/x", "T-7", "agent")
	r.SetTests(10, 10)
	r.SetSentrux(7000, 7020)
	r.SetMCPHealth(true)
	r.Evaluate()

	assert.True(t, r.IsEvoSpineCandidate())
}

func TestIsEvoSpineCandidate_NoImprovement(t *testing.T) {
	r := evalrecord.New("feat/x", "T-8", "agent")
	r.SetTests(10, 10)
	r.SetSentrux(7000, 7000)
	r.SetMCPHealth(true)
	r.Evaluate()

	assert.False(t, r.IsEvoSpineCandidate())
}
