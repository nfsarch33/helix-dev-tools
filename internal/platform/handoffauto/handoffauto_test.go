package handoffauto

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateHandoffMarkdown(t *testing.T) {
	doc := HandoffDoc{
		FromAgent: "cursor-parent",
		ToAgent:   "claude-code",
		SprintID:  "v6350",
		Tickets: []TicketEvidence{
			{TicketID: "T-1", Title: "Tunnel fix", Status: "done", Evidence: "healthz ok"},
			{TicketID: "T-2", Title: "Search fix", Status: "blocked", Evidence: ""},
		},
		Context:     "Mem0 search needs embedding pipeline fix",
		GeneratedAt: time.Date(2026, 5, 19, 22, 0, 0, 0, time.FixedZone("AEST", 10*3600)),
	}

	md := GenerateHandoffMarkdown(doc)

	if !strings.Contains(md, "cursor-parent -> claude-code") {
		t.Error("missing from/to agents")
	}
	if !strings.Contains(md, "v6350") {
		t.Error("missing sprint ID")
	}
	if !strings.Contains(md, "T-1") {
		t.Error("missing completed ticket")
	}
	if !strings.Contains(md, "T-2") {
		t.Error("missing blocked ticket")
	}
	if !strings.Contains(md, "embedding pipeline") {
		t.Error("missing context")
	}
}

func TestGenerateKickoffPrompt(t *testing.T) {
	tickets := []TicketEvidence{
		{TicketID: "T-1", Title: "Done task", Status: "done", AgentID: "agent-a"},
		{TicketID: "T-2", Title: "Pending task", Status: "backlog", AgentID: "agent-b"},
		{TicketID: "T-3", Title: "Unassigned", Status: "backlog", AgentID: ""},
	}

	prompt := GenerateKickoffPrompt("v6350", "agent-b", tickets)

	if !strings.Contains(prompt, "T-2") {
		t.Error("missing assigned ticket")
	}
	if !strings.Contains(prompt, "T-3") {
		t.Error("missing unassigned ticket")
	}
	if strings.Contains(prompt, "T-1") {
		t.Error("should not include done ticket")
	}
	if !strings.Contains(prompt, "task_claim") {
		t.Error("missing workflow instructions")
	}
}

func TestShouldAutoHandoff_AboveThreshold(t *testing.T) {
	tickets := []TicketEvidence{
		{Status: "done"}, {Status: "done"}, {Status: "done"},
		{Status: "done"}, {Status: "backlog"},
	}
	if !ShouldAutoHandoff(tickets, 0.8) {
		t.Error("expected handoff at 80% threshold (4/5 = 80%)")
	}
}

func TestShouldAutoHandoff_BelowThreshold(t *testing.T) {
	tickets := []TicketEvidence{
		{Status: "done"}, {Status: "done"},
		{Status: "backlog"}, {Status: "backlog"}, {Status: "backlog"},
	}
	if ShouldAutoHandoff(tickets, 0.8) {
		t.Error("should not handoff at 40% (below 80% threshold)")
	}
}

func TestShouldAutoHandoff_EmptyTickets(t *testing.T) {
	if ShouldAutoHandoff(nil, 0.8) {
		t.Error("should not handoff with no tickets")
	}
}

func TestGenerateHandoffMarkdown_AllDone(t *testing.T) {
	doc := HandoffDoc{
		FromAgent: "a",
		ToAgent:   "b",
		SprintID:  "v1",
		Tickets: []TicketEvidence{
			{TicketID: "T-1", Title: "Task 1", Status: "done", Evidence: "passed"},
		},
		GeneratedAt: time.Now(),
	}
	md := GenerateHandoffMarkdown(doc)
	if !strings.Contains(md, "## Completed Tickets") {
		t.Error("missing completed section")
	}
}
