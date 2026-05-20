package sprintdispatch

import (
	"fmt"
	"strings"
)

const (
	TargetClaudeCode = "claude-code"
	TargetCodex      = "codex"
)

type Ticket struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

type DispatchConfig struct {
	SprintID  string
	Target    string
	Workspace string
	Hours     int
}

func (c DispatchConfig) Validate() error {
	if c.SprintID == "" {
		return fmt.Errorf("sprint ID is required")
	}
	if c.Target == "" {
		return fmt.Errorf("target agent is required")
	}
	if c.Hours <= 0 {
		return fmt.Errorf("hours must be positive")
	}
	return nil
}

func GenerateKickoff(cfg DispatchConfig, tickets []Ticket) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are %s, working on sprint %s.\n\n", cfg.Target, cfg.SprintID)

	if len(tickets) == 0 {
		b.WriteString("No tickets assigned yet. Check sprintboard for available work.\n")
	} else {
		b.WriteString("## Your Tickets\n\n")
		for _, t := range tickets {
			fmt.Fprintf(&b, "- %s: %s [%s]\n", t.ID, t.Title, t.Status)
		}
	}

	b.WriteString("\n## Race Prevention Protocol\n\n")
	b.WriteString("Before starting ANY work:\n")
	fmt.Fprintf(&b, "1. Check sprintboard: sprint_status(sprint_id=%q)\n", cfg.SprintID)
	fmt.Fprintf(&b, "2. Register: agent_register(surface=%q)\n", cfg.Target)
	b.WriteString("3. Claim ticket: task_claim(ticket_id=\"<ticket>\")\n")
	b.WriteString("4. If claim returns conflict, pick a different ticket\n")
	b.WriteString("5. On completion: task_complete(ticket_id=\"<ticket>\")\n")
	b.WriteString("6. Handoff: handoff_publish(ticket_id=\"<ticket>\", to_agent=\"cursor-parent\")\n")

	if cfg.Hours > 0 {
		fmt.Fprintf(&b, "\nCRITICAL: Work for %d+ hours. Claim via sprintboard before any code change.\n", cfg.Hours)
	}

	return b.String()
}

func BuildCommand(cfg DispatchConfig, prompt string) []string {
	switch cfg.Target {
	case TargetClaudeCode:
		args := []string{"claude", "-p"}
		if cfg.Workspace != "" {
			args = append(args, "--add-dir", cfg.Workspace)
		}
		args = append(args, prompt)
		return args
	case TargetCodex:
		args := []string{"codex", "exec", "--skip-git-repo-check"}
		args = append(args, prompt)
		return args
	default:
		return nil
	}
}
