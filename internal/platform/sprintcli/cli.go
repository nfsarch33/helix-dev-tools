package sprintcli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nfsarch33/helix-dev-tools/internal/platform/agentidentity"
	"github.com/nfsarch33/helix-dev-tools/internal/platform/handoffgen"
	"github.com/nfsarch33/helix-dev-tools/internal/platform/sprintboard"
)

type CLI struct {
	store *sprintboard.Store
}

func New(dbPath string) (*CLI, error) {
	store, err := sprintboard.Open(dbPath)
	if err != nil {
		return nil, err
	}
	return &CLI{store: store}, nil
}

func (c *CLI) Close() error {
	return c.store.Close()
}

func (c *CLI) CreateSprint(id, name, theme string) (string, error) {
	agent := agentidentity.Resolve()
	err := c.store.CreateSprint(sprintboard.Sprint{
		ID:         id,
		Name:       name,
		Theme:      theme,
		OwnerAgent: string(agent.ID),
		Status:     sprintboard.SprintPlanned,
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Sprint %q created (owner: %s, theme: %s)", id, agent.ID, theme), nil
}

func (c *CLI) ListSprints() (string, error) {
	sprints, err := c.store.ListSprints()
	if err != nil {
		return "", err
	}
	if len(sprints) == 0 {
		return "No sprints found.", nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("%-12s %-8s %-15s %s\n", "ID", "Status", "Owner", "Name"))
	b.WriteString(strings.Repeat("-", 60) + "\n")
	for _, sp := range sprints {
		b.WriteString(fmt.Sprintf("%-12s %-8s %-15s %s\n", sp.ID, sp.Status, sp.OwnerAgent, sp.Name))
	}
	return b.String(), nil
}

func (c *CLI) SprintStatus(id string) (string, error) {
	summary, err := c.store.SprintSummary(id)
	if err != nil {
		return "", err
	}
	data, _ := json.MarshalIndent(summary, "", "  ")
	return string(data), nil
}

func (c *CLI) AssignTicket(ticketID, agent string) (string, error) {
	if err := c.store.AssignTicket(ticketID, agent); err != nil {
		return "", err
	}
	return fmt.Sprintf("Ticket %q assigned to %s", ticketID, agent), nil
}

func (c *CLI) GenerateKickoff(sprintID, agent string) (string, error) {
	summary, err := c.store.SprintSummary(sprintID)
	if err != nil {
		return "", err
	}

	tickets, err := c.store.ListTickets(sprintID)
	if err != nil {
		return "", err
	}

	var agentTickets []sprintboard.Ticket
	for _, t := range tickets {
		if t.OwnerAgent == agent {
			agentTickets = append(agentTickets, t)
		}
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Sprint Kickoff: %s\n\n", summary.Sprint.Name))
	b.WriteString(fmt.Sprintf("> Agent: %s\n", agent))
	b.WriteString(fmt.Sprintf("> Sprint: %s (theme: %s)\n", sprintID, summary.Sprint.Theme))
	b.WriteString(fmt.Sprintf("> Total tickets in sprint: %d\n", summary.TotalTickets))
	b.WriteString(fmt.Sprintf("> Your tickets: %d\n\n", len(agentTickets)))

	if len(agentTickets) == 0 {
		b.WriteString("No tickets assigned to you in this sprint.\n")
		return b.String(), nil
	}

	b.WriteString("## Assigned Work\n\n")
	for i, t := range agentTickets {
		b.WriteString(fmt.Sprintf("%d. **%s** [%s] (priority: %d)\n", i+1, t.Title, t.Status, t.Priority))
		if t.Description != "" {
			b.WriteString(fmt.Sprintf("   %s\n", t.Description))
		}
		if t.AcceptanceCriteria != "" {
			b.WriteString(fmt.Sprintf("   AC: %s\n", t.AcceptanceCriteria))
		}
	}

	b.WriteString("\n## Instructions\n\n")
	b.WriteString("1. Read daily-startup-prompt.md first\n")
	b.WriteString("2. Run workspace doctor to verify state\n")
	b.WriteString("3. Pick up your first ticket and mark in_progress\n")
	b.WriteString("4. Follow TDD: RED test -> GREEN impl -> Sentrux gate\n")
	b.WriteString("5. On completion, mark done and create handoff if needed\n")

	return b.String(), nil
}

func (c *CLI) GenerateHandoff(ticketID, toAgent string) (string, error) {
	ticket, err := c.store.GetTicket(ticketID)
	if err != nil {
		return "", err
	}

	agent := agentidentity.Resolve()
	h := handoffgen.Generate(handoffgen.HandoffContext{
		TicketID:    ticketID,
		TicketTitle: ticket.Title,
		SprintID:    ticket.SprintID,
		FromAgent:   string(agent.ID),
		ToAgent:     toAgent,
		Status:      string(ticket.Status),
		Timestamp:   time.Now(),
	})

	return h.Content, nil
}
