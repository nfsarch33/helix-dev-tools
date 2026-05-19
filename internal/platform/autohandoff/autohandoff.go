package autohandoff

import (
	"fmt"
	"strings"
	"time"
)

type TicketStatus string

const (
	StatusDone    TicketStatus = "done"
	StatusBlocked TicketStatus = "blocked"
	StatusOpen    TicketStatus = "open"
)

type Ticket struct {
	ID     string
	Title  string
	Status TicketStatus
}

type Sprint struct {
	Name    string
	Tickets []Ticket
}

func OnSprintClose(sprint Sprint) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Handoff: %s\n\n", sprint.Name))
	b.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().Format(time.RFC3339)))

	var done, blocked, open []Ticket
	for _, t := range sprint.Tickets {
		switch t.Status {
		case StatusDone:
			done = append(done, t)
		case StatusBlocked:
			blocked = append(blocked, t)
		default:
			open = append(open, t)
		}
	}

	b.WriteString(fmt.Sprintf("## Summary\n\n- Completed: %d\n- Blocked: %d\n- Open: %d\n\n", len(done), len(blocked), len(open)))

	if len(done) > 0 {
		b.WriteString("## Completed\n\n")
		for _, t := range done {
			b.WriteString(fmt.Sprintf("- [%s] %s\n", t.ID, t.Title))
		}
		b.WriteString("\n")
	}

	if len(blocked) > 0 {
		b.WriteString("## Blocked\n\n")
		for _, t := range blocked {
			b.WriteString(fmt.Sprintf("- [%s] %s\n", t.ID, t.Title))
		}
		b.WriteString("\n")
	}

	if len(open) > 0 {
		b.WriteString("## Carry-Forward\n\n")
		for _, t := range open {
			b.WriteString(fmt.Sprintf("- [%s] %s\n", t.ID, t.Title))
		}
		b.WriteString("\n")
	}

	return b.String()
}
