package sprintcli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/platform/sprintboard"
)

func testCLI(t *testing.T) *CLI {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	c, err := New(dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func TestCreateSprint(t *testing.T) {
	c := testCLI(t)
	msg, err := c.CreateSprint("v6120", "Next Sprint", "platform")
	if err != nil {
		t.Fatalf("CreateSprint: %v", err)
	}
	if !strings.Contains(msg, "v6120") {
		t.Errorf("expected v6120 in message, got %q", msg)
	}
}

func TestListSprints(t *testing.T) {
	c := testCLI(t)
	c.CreateSprint("v6120", "Sprint A", "platform")
	c.CreateSprint("v6122", "Sprint B", "infra")

	output, err := c.ListSprints()
	if err != nil {
		t.Fatalf("ListSprints: %v", err)
	}
	if !strings.Contains(output, "v6120") {
		t.Error("missing v6120")
	}
	if !strings.Contains(output, "v6122") {
		t.Error("missing v6122")
	}
}

func TestListSprintsEmpty(t *testing.T) {
	c := testCLI(t)
	output, _ := c.ListSprints()
	if !strings.Contains(output, "No sprints") {
		t.Errorf("expected 'No sprints', got %q", output)
	}
}

func TestSprintStatus(t *testing.T) {
	c := testCLI(t)
	c.CreateSprint("v6120", "Test", "test")
	output, err := c.SprintStatus("v6120")
	if err != nil {
		t.Fatalf("SprintStatus: %v", err)
	}
	if !strings.Contains(output, "v6120") {
		t.Error("missing sprint ID in output")
	}
}

func TestGenerateKickoff(t *testing.T) {
	c := testCLI(t)
	t.Setenv("CURSOR", "1")

	c.CreateSprint("v6120", "Platform Sprint", "platform")
	c.store.CreateTicket(sprintboard.Ticket{
		ID: "t-1", SprintID: "v6120", Title: "Build embedding client",
		OwnerAgent: "cursor-parent", Priority: 5, Description: "HTTP client for embeddings",
	})
	c.store.CreateTicket(sprintboard.Ticket{
		ID: "t-2", SprintID: "v6120", Title: "Fix mem0 timeout",
		OwnerAgent: "codex", Priority: 3,
	})

	output, err := c.GenerateKickoff("v6120", "cursor-parent")
	if err != nil {
		t.Fatalf("GenerateKickoff: %v", err)
	}
	if !strings.Contains(output, "Build embedding client") {
		t.Error("missing assigned ticket")
	}
	if strings.Contains(output, "Fix mem0 timeout") {
		t.Error("should not show other agent's ticket")
	}
	if !strings.Contains(output, "Your tickets: 1") {
		t.Error("expected 1 ticket assigned")
	}
}

func TestGenerateKickoffNoTickets(t *testing.T) {
	c := testCLI(t)
	c.CreateSprint("v6120", "Empty Sprint", "test")

	output, err := c.GenerateKickoff("v6120", "codex")
	if err != nil {
		t.Fatalf("GenerateKickoff: %v", err)
	}
	if !strings.Contains(output, "No tickets assigned") {
		t.Error("should indicate no tickets")
	}
}

func TestAssignTicket(t *testing.T) {
	c := testCLI(t)
	c.store.CreateTicket(sprintboard.Ticket{ID: "t-1", Title: "Task"})

	msg, err := c.AssignTicket("t-1", "claude-code")
	if err != nil {
		t.Fatalf("AssignTicket: %v", err)
	}
	if !strings.Contains(msg, "claude-code") {
		t.Error("expected agent name in message")
	}
}
