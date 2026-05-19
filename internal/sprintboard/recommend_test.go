package sprintboard_test

import (
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/multiagent"
	"github.com/nfsarch33/helix-dev-tools/internal/sprintboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecommendTasks_Basic(t *testing.T) {
	agents := []multiagent.AgentProfile{
		{ID: "cursor-parent", Capabilities: []string{"go", "tdd", "infra"}, MaxLoad: 5},
	}
	tickets := []multiagent.Ticket{
		{ID: "T-001", RequiredCaps: []string{"go"}, Priority: 10},
		{ID: "T-002", RequiredCaps: []string{"flutter"}, Priority: 8},
	}
	rec := sprintboard.RecommendTasks("cursor-parent", agents, tickets, 5)
	require.Len(t, rec, 1)
	assert.Equal(t, "T-001", rec[0].ID)
}

func TestRecommendTasks_MultiAgent(t *testing.T) {
	agents := []multiagent.AgentProfile{
		{ID: "cursor-parent", Capabilities: []string{"go", "tdd"}, MaxLoad: 3},
		{ID: "claude-code", Capabilities: []string{"go", "typescript"}, MaxLoad: 3},
	}
	tickets := []multiagent.Ticket{
		{ID: "T-001", RequiredCaps: []string{"typescript"}, Priority: 10},
		{ID: "T-002", RequiredCaps: []string{"go"}, Priority: 5},
	}
	rec := sprintboard.RecommendTasks("claude-code", agents, tickets, 5)
	require.NotEmpty(t, rec)
	assert.Equal(t, "T-001", rec[0].ID)
}

func TestDistributeTasks_Balanced(t *testing.T) {
	agents := []multiagent.AgentProfile{
		{ID: "cursor-parent", Capabilities: []string{"go", "infra"}, MaxLoad: 2},
		{ID: "claude-code", Capabilities: []string{"go", "typescript"}, MaxLoad: 2},
		{ID: "codex", Capabilities: []string{"review", "docs"}, MaxLoad: 3},
	}
	tickets := []multiagent.Ticket{
		{ID: "T-001", RequiredCaps: []string{"go"}, Priority: 10},
		{ID: "T-002", RequiredCaps: []string{"typescript"}, Priority: 9},
		{ID: "T-003", RequiredCaps: []string{"review"}, Priority: 8},
		{ID: "T-004", RequiredCaps: []string{"docs"}, Priority: 7},
	}
	assignments := sprintboard.DistributeTasks(agents, tickets)
	assert.NotEmpty(t, assignments)
	assert.Equal(t, "codex", assignments["T-003"])
	assert.Equal(t, "codex", assignments["T-004"])
	assert.Equal(t, "claude-code", assignments["T-002"])
}
