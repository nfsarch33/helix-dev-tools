package fanout_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/fanout"
	"github.com/nfsarch33/helix-dev-tools/internal/platform/sprintboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestLoadOwnerManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "owners.yaml")
	content := `cursor-global-kb: cursor-parent
cursor-tools: cursor-parent
ecommerce: codex-ec
resume: codex-jobhunt
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	manifest, err := fanout.LoadOwnerManifest(path)
	require.NoError(t, err)
	assert.Equal(t, "cursor-parent", manifest.OwnerOf("cursor-tools"))
	assert.Equal(t, "codex-ec", manifest.OwnerOf("ecommerce"))
	assert.Equal(t, "", manifest.OwnerOf("unknown-repo"))
}

func TestLoadOwnerManifest_NotFound(t *testing.T) {
	_, err := fanout.LoadOwnerManifest("/nonexistent/path.yaml")
	assert.Error(t, err)
}

func TestDefaultAgentProfiles(t *testing.T) {
	profiles := fanout.DefaultAgentProfiles()
	assert.Len(t, profiles, 3)

	cp := profiles["cursor-parent"]
	assert.Contains(t, cp.Capabilities, "infra")
	assert.Contains(t, cp.Capabilities, "coordination")

	cc := profiles["claude-code"]
	assert.Contains(t, cc.Capabilities, "helixon")

	cx := profiles["codex"]
	assert.Contains(t, cx.Capabilities, "ec-product")
}

func TestAssignTickets_UnassignedGetAssigned(t *testing.T) {
	tickets := []sprintboard.Ticket{
		{ID: "t1", Title: "Fix infra bug", Description: "coordination infra work", Priority: 3},
		{ID: "t2", Title: "Helixon migration", Description: "helixon platform adapter", Priority: 2},
		{ID: "t3", Title: "EC product page", Description: "ec product feature", Priority: 1},
	}

	profiles := fanout.DefaultAgentProfiles()
	engine := fanout.NewEngine(profiles)

	assignments := engine.AssignTickets(tickets)
	assert.Len(t, assignments, 3)

	assignMap := make(map[string]string)
	for _, a := range assignments {
		assignMap[a.TicketID] = a.AgentID
	}

	assert.Equal(t, "cursor-parent", assignMap["t1"])
	assert.Equal(t, "claude-code", assignMap["t2"])
	assert.Equal(t, "codex", assignMap["t3"])
}

func TestAssignTickets_AlreadyAssigned(t *testing.T) {
	tickets := []sprintboard.Ticket{
		{ID: "t1", Title: "Already owned", OwnerAgent: "existing-agent", Priority: 1},
		{ID: "t2", Title: "Unassigned infra", Description: "infra work", Priority: 2},
	}

	profiles := fanout.DefaultAgentProfiles()
	engine := fanout.NewEngine(profiles)

	assignments := engine.AssignTickets(tickets)
	assert.Len(t, assignments, 1)
	assert.Equal(t, "t2", assignments[0].TicketID)
}

func TestAssignTickets_PriorityOrder(t *testing.T) {
	tickets := []sprintboard.Ticket{
		{ID: "low", Title: "Low priority", Description: "infra task", Priority: 1},
		{ID: "high", Title: "High priority", Description: "critical infra", Priority: 10},
		{ID: "mid", Title: "Mid priority", Description: "infra fix", Priority: 5},
	}

	profiles := fanout.DefaultAgentProfiles()
	engine := fanout.NewEngine(profiles)

	assignments := engine.AssignTickets(tickets)
	assert.Equal(t, "high", assignments[0].TicketID)
	assert.Equal(t, "mid", assignments[1].TicketID)
	assert.Equal(t, "low", assignments[2].TicketID)
}

func TestAssignTickets_Empty(t *testing.T) {
	profiles := fanout.DefaultAgentProfiles()
	engine := fanout.NewEngine(profiles)

	assignments := engine.AssignTickets(nil)
	assert.Empty(t, assignments)
}

func TestMatchAgent_Keywords(t *testing.T) {
	profiles := fanout.DefaultAgentProfiles()
	engine := fanout.NewEngine(profiles)

	tests := []struct {
		title       string
		desc        string
		expectAgent string
	}{
		{"Fix coordination signals", "infra coordination work", "cursor-parent"},
		{"Sprint scaffold update", "sprint tooling", "cursor-parent"},
		{"Helixon adapter migration", "helixon platform engram", "claude-code"},
		{"Engram memory fix", "engram adapter", "claude-code"},
		{"EC checkout flow", "ecommerce product", "codex"},
		{"Product page styling", "ec product design", "codex"},
		{"Vague task", "no keywords", "cursor-parent"},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			ticket := sprintboard.Ticket{
				ID:          "test",
				Title:       tt.title,
				Description: tt.desc,
			}
			agent := engine.MatchAgent(ticket)
			assert.Equal(t, tt.expectAgent, agent, "for: %s", tt.title)
		})
	}
}

func TestGenerateHandoffDoc(t *testing.T) {
	assignment := fanout.Assignment{
		TicketID:    "v6505-1",
		AgentID:     "claude-code",
		TicketTitle: "Helixon platform migration",
		TicketDesc:  "Migrate 813 files from IronClaw to Helixon module paths",
		Priority:    3,
	}

	doc := fanout.GenerateHandoffDoc(assignment)
	assert.Contains(t, doc, "v6505-1")
	assert.Contains(t, doc, "claude-code")
	assert.Contains(t, doc, "Helixon platform migration")
	assert.Contains(t, doc, "813 files")
}

func TestGenerateKickoffPrompt(t *testing.T) {
	assignment := fanout.Assignment{
		TicketID:    "v6504-1",
		AgentID:     "cursor-parent",
		TicketTitle: "Token tracking enhancement",
		TicketDesc:  "Add per-provider token tracking to agentrace",
		Priority:    5,
	}

	prompt := fanout.GenerateKickoffPrompt(assignment, []string{"Mem0 search works", "MiniMax key rotation done"})
	assert.Contains(t, prompt, "v6504-1")
	assert.Contains(t, prompt, "Token tracking")
	assert.Contains(t, prompt, "Mem0 search works")
	assert.Contains(t, prompt, "cursor-parent")
}

func TestGenerateKickoffPrompt_NoMemories(t *testing.T) {
	assignment := fanout.Assignment{
		TicketID:    "t1",
		AgentID:     "codex",
		TicketTitle: "Test",
		TicketDesc:  "Test desc",
		Priority:    1,
	}

	prompt := fanout.GenerateKickoffPrompt(assignment, nil)
	assert.Contains(t, prompt, "t1")
	assert.NotContains(t, prompt, "Recent context")
}
