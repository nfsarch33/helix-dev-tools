package sessionhook

import (
	"fmt"
	"os"
	"time"

	"github.com/nfsarch33/helix-dev-tools/internal/platform/agentidentity"
	"github.com/nfsarch33/helix-dev-tools/internal/platform/handoffgen"
	"github.com/nfsarch33/helix-dev-tools/internal/platform/sprintboard"
)

type SessionAction string

const (
	ActionStart SessionAction = "start"
	ActionStop  SessionAction = "stop"
)

type HookResult struct {
	TicketID    string `json:"ticket_id,omitempty"`
	HandoffPath string `json:"handoff_path,omitempty"`
	Action      string `json:"action"`
}

func RunSessionHook(action SessionAction, dbPath string) (HookResult, error) {
	if os.Getenv("AGENTRACE_ENABLED") == "false" {
		return HookResult{Action: string(action)}, nil
	}

	store, err := sprintboard.Open(dbPath)
	if err != nil {
		return HookResult{}, fmt.Errorf("open sprint board: %w", err)
	}
	defer store.Close()

	agent := agentidentity.Resolve()
	sessionID := fmt.Sprintf("session-%s-%d", agent.ID, time.Now().Unix())

	switch action {
	case ActionStart:
		return handleStart(store, agent, sessionID)
	case ActionStop:
		return handleStop(store, agent, sessionID)
	default:
		return HookResult{}, fmt.Errorf("unknown action: %s", action)
	}
}

func handleStart(store *sprintboard.Store, agent agentidentity.AgentInfo, sessionID string) (HookResult, error) {
	err := store.CreateTicket(sprintboard.Ticket{
		ID:         sessionID,
		Title:      fmt.Sprintf("Session: %s (%s)", agent.ID, time.Now().Format("2006-01-02 15:04")),
		Status:     sprintboard.StatusInProgress,
		OwnerAgent: string(agent.ID),
	})
	if err != nil {
		return HookResult{}, err
	}

	return HookResult{
		TicketID: sessionID,
		Action:   "start",
	}, nil
}

func handleStop(store *sprintboard.Store, agent agentidentity.AgentInfo, sessionID string) (HookResult, error) {
	tickets, err := store.ListTickets("")
	if err != nil {
		return HookResult{}, err
	}

	var activeTicket *sprintboard.Ticket
	for i := range tickets {
		if tickets[i].OwnerAgent == string(agent.ID) && tickets[i].Status == sprintboard.StatusInProgress {
			activeTicket = &tickets[i]
			break
		}
	}

	result := HookResult{Action: "stop"}

	if activeTicket != nil {
		store.UpdateTicket(activeTicket.ID, sprintboard.StatusReview, string(agent.ID), "session ended")
		result.TicketID = activeTicket.ID

		handoff := handoffgen.Generate(handoffgen.HandoffContext{
			TicketID:    activeTicket.ID,
			TicketTitle: activeTicket.Title,
			FromAgent:   string(agent.ID),
			ToAgent:     "next-session",
			Status:      "session_ended",
			Timestamp:   time.Now(),
		})
		result.HandoffPath = handoff.Filename
	}

	return result, nil
}
