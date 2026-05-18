package handoffgen

import (
	"fmt"
	"strings"
	"time"
)

type HandoffContext struct {
	TicketID    string
	TicketTitle string
	SprintID    string
	FromAgent   string
	ToAgent     string
	Status      string
	WorkDone    string
	CarryItems  []string
	Blockers    []string
	FilesChanged []string
	Timestamp   time.Time
}

type GeneratedHandoff struct {
	Filename string
	Content  string
}

func Generate(ctx HandoffContext) GeneratedHandoff {
	if ctx.Timestamp.IsZero() {
		ctx.Timestamp = time.Now()
	}

	date := ctx.Timestamp.Format("2006-01-02")
	filename := fmt.Sprintf("session-handoffs/%s-%s-handoff.md", date, sanitizeFilename(ctx.TicketID))

	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Handoff: %s\n\n", ctx.TicketTitle))
	b.WriteString(fmt.Sprintf("> Created: %s\n", ctx.Timestamp.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("> Ticket: %s\n", ctx.TicketID))
	b.WriteString(fmt.Sprintf("> Sprint: %s\n", ctx.SprintID))
	b.WriteString(fmt.Sprintf("> From: %s -> To: %s\n\n", ctx.FromAgent, ctx.ToAgent))

	b.WriteString("## Status\n\n")
	b.WriteString(fmt.Sprintf("Current status: **%s**\n\n", ctx.Status))

	if ctx.WorkDone != "" {
		b.WriteString("## Work Completed\n\n")
		b.WriteString(ctx.WorkDone)
		b.WriteString("\n\n")
	}

	if len(ctx.FilesChanged) > 0 {
		b.WriteString("## Files Changed\n\n")
		for _, f := range ctx.FilesChanged {
			b.WriteString(fmt.Sprintf("- `%s`\n", f))
		}
		b.WriteString("\n")
	}

	if len(ctx.CarryItems) > 0 {
		b.WriteString("## Carry-Forward Items\n\n")
		for i, item := range ctx.CarryItems {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, item))
		}
		b.WriteString("\n")
	}

	if len(ctx.Blockers) > 0 {
		b.WriteString("## Blockers\n\n")
		for _, blocker := range ctx.Blockers {
			b.WriteString(fmt.Sprintf("- %s\n", blocker))
		}
		b.WriteString("\n")
	}

	b.WriteString("## Next Steps for Receiving Agent\n\n")
	b.WriteString(fmt.Sprintf("1. Read this handoff document\n"))
	b.WriteString(fmt.Sprintf("2. Query sprint board: `sprint_status %s`\n", ctx.SprintID))
	b.WriteString(fmt.Sprintf("3. Pick up ticket: `ticket_update %s in_progress`\n", ctx.TicketID))
	b.WriteString(fmt.Sprintf("4. Review files changed above\n"))
	b.WriteString(fmt.Sprintf("5. Address carry-forward items and blockers\n"))

	return GeneratedHandoff{
		Filename: filename,
		Content:  b.String(),
	}
}

func sanitizeFilename(s string) string {
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		" ", "-",
		":", "-",
	)
	return strings.ToLower(replacer.Replace(s))
}
