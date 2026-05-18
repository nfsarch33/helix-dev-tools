package multiagent_test

import (
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/multiagent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentProfile_Capabilities(t *testing.T) {
	p := multiagent.AgentProfile{
		ID:           "cursor-parent",
		Surface:      "cursor",
		Capabilities: []string{"go", "tdd", "infra", "docs"},
		MaxLoad:      3,
	}
	assert.True(t, p.HasCapability("go"))
	assert.True(t, p.HasCapability("tdd"))
	assert.False(t, p.HasCapability("flutter"))
}

func TestDistributor_EmptyAgents(t *testing.T) {
	d := multiagent.NewDistributor()
	tickets := []multiagent.Ticket{
		{ID: "T-001", RequiredCaps: []string{"go"}},
	}
	assignments := d.Distribute(tickets)
	assert.Empty(t, assignments)
}

func TestDistributor_SingleAgent(t *testing.T) {
	d := multiagent.NewDistributor()
	d.RegisterAgent(multiagent.AgentProfile{
		ID:           "cursor-parent",
		Surface:      "cursor",
		Capabilities: []string{"go", "tdd", "infra"},
		MaxLoad:      2,
	})
	tickets := []multiagent.Ticket{
		{ID: "T-001", RequiredCaps: []string{"go"}},
		{ID: "T-002", RequiredCaps: []string{"tdd"}},
		{ID: "T-003", RequiredCaps: []string{"go"}},
	}
	assignments := d.Distribute(tickets)
	assert.Len(t, assignments, 2)
	assert.Equal(t, "cursor-parent", assignments["T-001"])
	assert.Equal(t, "cursor-parent", assignments["T-002"])
}

func TestDistributor_MultiAgent(t *testing.T) {
	d := multiagent.NewDistributor()
	d.RegisterAgent(multiagent.AgentProfile{
		ID: "cursor-parent", Surface: "cursor",
		Capabilities: []string{"go", "tdd", "infra"}, MaxLoad: 2,
	})
	d.RegisterAgent(multiagent.AgentProfile{
		ID: "claude-code", Surface: "vscode",
		Capabilities: []string{"go", "typescript", "testing"}, MaxLoad: 2,
	})
	d.RegisterAgent(multiagent.AgentProfile{
		ID: "codex", Surface: "codex-cli",
		Capabilities: []string{"review", "docs", "architecture"}, MaxLoad: 3,
	})

	tickets := []multiagent.Ticket{
		{ID: "T-001", RequiredCaps: []string{"go", "tdd"}},
		{ID: "T-002", RequiredCaps: []string{"typescript"}},
		{ID: "T-003", RequiredCaps: []string{"review"}},
		{ID: "T-004", RequiredCaps: []string{"go"}},
		{ID: "T-005", RequiredCaps: []string{"docs"}},
	}
	assignments := d.Distribute(tickets)
	require.NotEmpty(t, assignments)
	assert.Equal(t, "claude-code", assignments["T-002"])
	assert.Equal(t, "codex", assignments["T-003"])
	assert.Equal(t, "codex", assignments["T-005"])
}

func TestDistributor_CapabilityMatch(t *testing.T) {
	d := multiagent.NewDistributor()
	d.RegisterAgent(multiagent.AgentProfile{
		ID: "cursor-parent", Capabilities: []string{"go", "infra"}, MaxLoad: 5,
	})
	d.RegisterAgent(multiagent.AgentProfile{
		ID: "codex", Capabilities: []string{"review", "docs"}, MaxLoad: 5,
	})

	tickets := []multiagent.Ticket{
		{ID: "T-001", RequiredCaps: []string{"flutter"}},
	}
	assignments := d.Distribute(tickets)
	assert.Empty(t, assignments)
}

func TestRecommender_NextTask(t *testing.T) {
	r := multiagent.NewRecommender()
	r.RegisterAgent(multiagent.AgentProfile{
		ID: "cursor-parent", Capabilities: []string{"go", "tdd"}, MaxLoad: 3,
	})
	r.AddTickets([]multiagent.Ticket{
		{ID: "T-001", RequiredCaps: []string{"go"}, Priority: 10},
		{ID: "T-002", RequiredCaps: []string{"go"}, Priority: 5},
		{ID: "T-003", RequiredCaps: []string{"flutter"}, Priority: 10},
	})

	rec := r.Recommend("cursor-parent", 2)
	require.Len(t, rec, 2)
	assert.Equal(t, "T-001", rec[0].ID)
	assert.Equal(t, "T-002", rec[1].ID)
}

func TestRecommender_RespectsClaimed(t *testing.T) {
	r := multiagent.NewRecommender()
	r.RegisterAgent(multiagent.AgentProfile{
		ID: "cursor-parent", Capabilities: []string{"go"}, MaxLoad: 3,
	})
	r.AddTickets([]multiagent.Ticket{
		{ID: "T-001", RequiredCaps: []string{"go"}, Priority: 10, ClaimedBy: "claude-code"},
		{ID: "T-002", RequiredCaps: []string{"go"}, Priority: 5},
	})

	rec := r.Recommend("cursor-parent", 5)
	require.Len(t, rec, 1)
	assert.Equal(t, "T-002", rec[0].ID)
}

func TestHandoffTemplate_Generate(t *testing.T) {
	tmpl := multiagent.HandoffTemplate{
		FromAgent: "cursor-parent",
		ToAgent:   "claude-code",
		TicketID:  "T-001",
		Summary:   "Docker isolation implemented, tests pass",
		Branch:    "feat/v6300-overnight",
		Evidence:  "8/8 tests PASS, race clean",
	}
	md := tmpl.ToMarkdown()
	assert.Contains(t, md, "cursor-parent")
	assert.Contains(t, md, "claude-code")
	assert.Contains(t, md, "T-001")
	assert.Contains(t, md, "feat/v6300-overnight")
}
