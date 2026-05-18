package agentmonitor

import (
	"testing"
	"time"

	"github.com/nfsarch33/helix-dev-tools/internal/platform/sprintboard"
)

func TestRenderProgressBar_Empty(t *testing.T) {
	bar := RenderProgressBar(0, 10, 10)
	expected := "[          ]   0%"
	if bar != expected {
		t.Errorf("Expected %q, got %q", expected, bar)
	}
}

func TestRenderProgressBar_Full(t *testing.T) {
	bar := RenderProgressBar(10, 10, 10)
	expected := "[##########] 100%"
	if bar != expected {
		t.Errorf("Expected %q, got %q", expected, bar)
	}
}

func TestRenderProgressBar_Half(t *testing.T) {
	bar := RenderProgressBar(5, 10, 10)
	expected := "[#####-----]  50%"
	if bar != expected {
		t.Errorf("Expected %q, got %q", expected, bar)
	}
}

func TestRenderProgressBar_ZeroTotal(t *testing.T) {
	bar := RenderProgressBar(0, 0, 10)
	expected := "[          ]   0%"
	if bar != expected {
		t.Errorf("Expected %q, got %q", expected, bar)
	}
}

func TestSprintProgress_Fields(t *testing.T) {
	// Create mock store
	store, err := sprintboard.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create a sprint
	sprintID := "test-sprint"
	err = store.CreateSprint(sprintboard.Sprint{
		ID: sprintID,
		Name: "Test Sprint",
		Status: sprintboard.SprintActive,
	})
	if err != nil {
		t.Fatalf("Failed to create sprint: %v", err)
	}

	// Create some sample tickets
	tickets := []sprintboard.Ticket{
		{
			ID:         "ticket1",
			SprintID:   sprintID,
			Status:     sprintboard.StatusDone,
			OwnerAgent: "agent1",
		},
		{
			ID:         "ticket2",
			SprintID:   sprintID,
			Status:     sprintboard.StatusInProgress,
			OwnerAgent: "agent1",
		},
		{
			ID:         "ticket3",
			SprintID:   sprintID,
			Status:     sprintboard.StatusBlocked,
			OwnerAgent: "agent2",
		},
		{
			ID:         "ticket4",
			SprintID:   sprintID,
			Status:     sprintboard.StatusReview,
			OwnerAgent: "agent2",
		},
	}

	for _, ticket := range tickets {
		err = store.CreateTicket(ticket)
		if err != nil {
			t.Fatalf("Failed to create ticket: %v", err)
		}
	}

	// Create monitor
	monitor := NewMonitor(store)

	// Test sprint progress
	progress, err := monitor.SprintProgress(sprintID)
	if err != nil {
		t.Fatalf("Failed to get sprint progress: %v", err)
	}

	// Validate expectations
	if progress.Total != 4 {
		t.Errorf("Expected total tickets: 4, got: %d", progress.Total)
	}
	if progress.Done != 1 {
		t.Errorf("Expected done tickets: 1, got: %d", progress.Done)
	}
	if progress.Active != 2 {
		t.Errorf("Expected active tickets: 2, got: %d", progress.Active)
	}
	if progress.Blocked != 1 {
		t.Errorf("Expected blocked tickets: 1, got: %d", progress.Blocked)
	}
	if progress.ProgressPct != 25.0 {
		t.Errorf("Expected progress percentage: 25.0, got: %f", progress.ProgressPct)
	}
}

func TestAgentStatuses_EmptyStore(t *testing.T) {
	// Create mock store
	store, err := sprintboard.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create monitor
	monitor := NewMonitor(store)

	// Test agent statuses
	statuses, err := monitor.AgentStatuses()
	if err != nil {
		t.Fatalf("Failed to get agent statuses: %v", err)
	}

	if len(statuses) != 0 {
		t.Errorf("Expected 0 agent statuses, got: %d", len(statuses))
	}
}

func TestAgentStatuses_ActiveAgent(t *testing.T) {
	// Create mock store
	store, err := sprintboard.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Register an agent
	agent := sprintboard.Agent{
		ID:       "test-agent",
		Surface:  "test-surface",
		LastSeen: time.Now(),
	}
	err = store.RegisterAgent(agent)
	if err != nil {
		t.Fatalf("Failed to register agent: %v", err)
	}

	// Create some tickets
	sprintID := "test-sprint"
	tickets := []sprintboard.Ticket{
		{
			ID:         "ticket1",
			SprintID:   sprintID,
			Status:     sprintboard.StatusInProgress,
			OwnerAgent: "test-agent",
		},
		{
			ID:         "ticket2",
			SprintID:   sprintID,
			Status:     sprintboard.StatusDone,
			OwnerAgent: "another-agent",
		},
	}

	for _, ticket := range tickets {
		err = store.CreateTicket(ticket)
		if err != nil {
			t.Fatalf("Failed to create ticket: %v", err)
		}
	}

	// Create monitor
	monitor := NewMonitor(store)

	// Test agent statuses
	statuses, err := monitor.AgentStatuses()
	if err != nil {
		t.Fatalf("Failed to get agent statuses: %v", err)
	}

	if len(statuses) != 1 {
		t.Fatalf("Expected 1 agent status, got: %d", len(statuses))
	}

	status := statuses[0]
	if status.AgentID != "test-agent" {
		t.Errorf("Expected agent ID: test-agent, got: %s", status.AgentID)
	}
	if len(status.ActiveTickets) != 1 || status.ActiveTickets[0] != "ticket1" {
		t.Errorf("Expected 1 active ticket: ticket1, got: %v", status.ActiveTickets)
	}
	if status.IsExpired {
		t.Errorf("Expected agent to not be expired")
	}
}