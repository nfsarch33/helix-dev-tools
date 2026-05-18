package agentmonitor

import (
	"fmt"
	"time"

	"github.com/nfsarch33/helix-dev-tools/internal/platform/sprintboard"
)

type AgentStatus struct {
	AgentID      string
	ActiveTickets []string
	IdleMinutes  float64
	IsExpired    bool
}

type SprintProgress struct {
	SprintID    string
	Total       int
	Done        int
	Active      int
	Blocked     int
	ProgressPct float64
	ProgressBar string
}

type Monitor struct {
	store interface {
		ListActiveAgents() ([]sprintboard.Agent, error)
		ListTickets(sprintID string) ([]sprintboard.Ticket, error)
	}
}

// NewMonitor creates a new Monitor with the given store
func NewMonitor(store *sprintboard.Store) *Monitor {
	return &Monitor{store: store}
}

// AgentStatuses returns the status of all active agents
func (m *Monitor) AgentStatuses() ([]AgentStatus, error) {
	agents, err := m.store.ListActiveAgents()
	if err != nil {
		return nil, fmt.Errorf("list active agents: %w", err)
	}

	var statuses []AgentStatus
	now := time.Now()

	for _, agent := range agents {
		// Calculate idle minutes
		idleMinutes := now.Sub(agent.LastSeen).Minutes()
		isExpired := idleMinutes > 30

		// Find active tickets for this agent
		tickets, err := m.store.ListTickets("")
		if err != nil {
			return nil, fmt.Errorf("list tickets for agent %s: %w", agent.ID, err)
		}

		var activeTickets []string
		for _, ticket := range tickets {
			if ticket.OwnerAgent == agent.ID &&
				(ticket.Status == sprintboard.StatusInProgress ||
				 ticket.Status == sprintboard.StatusReview ||
				 ticket.Status == sprintboard.StatusReady) {
				activeTickets = append(activeTickets, ticket.ID)
			}
		}

		statuses = append(statuses, AgentStatus{
			AgentID:      agent.ID,
			ActiveTickets: activeTickets,
			IdleMinutes:  idleMinutes,
			IsExpired:    isExpired,
		})
	}

	return statuses, nil
}

// SprintProgress returns the progress of tickets in a given sprint
func (m *Monitor) SprintProgress(sprintID string) (SprintProgress, error) {
	tickets, err := m.store.ListTickets(sprintID)
	if err != nil {
		return SprintProgress{}, fmt.Errorf("list tickets for sprint %s: %w", sprintID, err)
	}

	var total, done, active, blocked int
	total = len(tickets)

	for _, ticket := range tickets {
		switch ticket.Status {
		case sprintboard.StatusDone:
			done++
		case sprintboard.StatusInProgress, sprintboard.StatusReady, sprintboard.StatusReview:
			active++
		case sprintboard.StatusBlocked:
			blocked++
		}
	}

	// Calculate progress percentage
	var progressPct float64
	if total > 0 {
		progressPct = float64(done) / float64(total) * 100
	}

	return SprintProgress{
		SprintID:     sprintID,
		Total:        total,
		Done:         done,
		Active:       active,
		Blocked:      blocked,
		ProgressPct:  progressPct,
		ProgressBar: RenderProgressBar(done, total, 10),
	}, nil
}

// RenderProgressBar creates a simple text-based progress bar using # for filled, - for empty.
func RenderProgressBar(done, total int, width int) string {
	if total <= 0 {
		empty := ""
		for i := 0; i < width; i++ {
			empty += " "
		}
		return fmt.Sprintf("[%s]   0%%", empty)
	}

	filledCount := int(float64(done) / float64(total) * float64(width))
	emptyCount := width - filledCount

	filled := ""
	for i := 0; i < filledCount; i++ {
		filled += "#"
	}
	// use '-' separator only when there are filled segments
	emptyChar := " "
	if filledCount > 0 {
		emptyChar = "-"
	}
	empty := ""
	for i := 0; i < emptyCount; i++ {
		empty += emptyChar
	}

	return fmt.Sprintf("[%s%s] %3d%%",
		filled,
		empty,
		int(float64(done)/float64(total)*100))
}