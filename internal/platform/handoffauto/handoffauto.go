package handoffauto

import (
	"fmt"
	"strings"
	"time"
)

type TicketEvidence struct {
	TicketID    string
	Title       string
	Status      string
	AgentID     string
	Evidence    string
	CompletedAt time.Time
}

type HandoffDoc struct {
	FromAgent   string
	ToAgent     string
	SprintID    string
	Tickets     []TicketEvidence
	Context     string
	GeneratedAt time.Time
}

func GenerateHandoffMarkdown(doc HandoffDoc) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Handoff: %s -> %s\n\n", doc.FromAgent, doc.ToAgent))
	sb.WriteString(fmt.Sprintf("**Sprint**: %s\n", doc.SprintID))
	sb.WriteString(fmt.Sprintf("**Generated**: %s\n\n", doc.GeneratedAt.Format(time.RFC3339)))

	sb.WriteString("## Completed Tickets\n\n")
	for _, t := range doc.Tickets {
		if t.Status == "done" {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n  - Evidence: %s\n", t.TicketID, t.Title, t.Evidence))
		}
	}

	sb.WriteString("\n## Pending/Blocked Tickets\n\n")
	for _, t := range doc.Tickets {
		if t.Status != "done" {
			sb.WriteString(fmt.Sprintf("- **%s** [%s]: %s\n", t.TicketID, t.Status, t.Title))
		}
	}

	if doc.Context != "" {
		sb.WriteString(fmt.Sprintf("\n## Context\n\n%s\n", doc.Context))
	}

	return sb.String()
}

func GenerateKickoffPrompt(sprintID, agentID string, tickets []TicketEvidence) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Kickoff: %s for %s\n\n", sprintID, agentID))
	sb.WriteString("## Your Tickets\n\n")

	for _, t := range tickets {
		if t.Status != "done" && (t.AgentID == agentID || t.AgentID == "") {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", t.TicketID, t.Title))
		}
	}

	sb.WriteString("\n## Workflow\n\n")
	sb.WriteString("1. `sprintboard: task_claim` before starting each ticket\n")
	sb.WriteString("2. TDD: failing test -> min code -> refactor\n")
	sb.WriteString("3. `sprintboard: task_complete` with evidence when done\n")
	sb.WriteString("4. Commit after each ticket\n")

	return sb.String()
}

func ShouldAutoHandoff(tickets []TicketEvidence, threshold float64) bool {
	if len(tickets) == 0 {
		return false
	}
	done := 0
	for _, t := range tickets {
		if t.Status == "done" {
			done++
		}
	}
	return float64(done)/float64(len(tickets)) >= threshold
}
