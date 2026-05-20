// Package fanout implements multi-agent ticket assignment for the
// sprint-fanout command. It reads sprintboard tickets and assigns
// unassigned ones to agents based on capability matching and priority.
package fanout

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/nfsarch33/helix-dev-tools/internal/platform/sprintboard"
	"gopkg.in/yaml.v3"
)

// OwnerManifest maps repo aliases to owner agent IDs.
type OwnerManifest struct {
	Entries map[string]string
}

// LoadOwnerManifest reads the owner manifest from a YAML file.
func LoadOwnerManifest(path string) (*OwnerManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read owner manifest: %w", err)
	}
	entries := make(map[string]string)
	if err := yaml.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse owner manifest: %w", err)
	}
	return &OwnerManifest{Entries: entries}, nil
}

// OwnerOf returns the agent ID that owns the given repo alias, or empty.
func (m *OwnerManifest) OwnerOf(alias string) string {
	if m == nil || m.Entries == nil {
		return ""
	}
	return m.Entries[alias]
}

// AgentProfile describes an agent's capabilities and keyword triggers.
type AgentProfile struct {
	ID           string
	Surface      string
	Capabilities []string
	Keywords     []string
}

// DefaultAgentProfiles returns the hardcoded agent capability map per
// the multi-agent SOP.
func DefaultAgentProfiles() map[string]AgentProfile {
	return map[string]AgentProfile{
		"cursor-parent": {
			ID:           "cursor-parent",
			Surface:      "cursor",
			Capabilities: []string{"infra", "coordination", "sprint", "tooling", "monitoring", "memory"},
			Keywords: []string{
				"infra", "coordination", "sprint", "scaffold", "token",
				"agentrace", "evospine", "monitoring", "daemon", "hook",
				"doctor", "identity", "worktree", "memory", "mem0",
				"signal", "fanout", "fleet", "daily", "report",
			},
		},
		"claude-code": {
			ID:           "claude-code",
			Surface:      "claude-code",
			Capabilities: []string{"helixon", "platform", "engram", "adapter", "migration"},
			Keywords: []string{
				"helixon", "engram", "adapter", "migration", "platform",
				"rebrand", "module", "import", "refactor",
			},
		},
		"codex": {
			ID:           "codex",
			Surface:      "codex",
			Capabilities: []string{"ec-product", "ecommerce", "frontend", "web"},
			Keywords: []string{
				"ec", "ecommerce", "product", "checkout", "cart",
				"storefront", "web", "frontend", "agentic",
			},
		},
	}
}

// Assignment is a ticket-to-agent assignment produced by the fanout engine.
type Assignment struct {
	TicketID    string `json:"ticket_id"`
	AgentID     string `json:"agent_id"`
	TicketTitle string `json:"ticket_title"`
	TicketDesc  string `json:"ticket_desc,omitempty"`
	Priority    int    `json:"priority"`
	Reason      string `json:"reason,omitempty"`
}

// Engine assigns tickets to agents using keyword matching.
type Engine struct {
	profiles map[string]AgentProfile
}

// NewEngine creates a fanout engine with the given agent profiles.
func NewEngine(profiles map[string]AgentProfile) *Engine {
	return &Engine{profiles: profiles}
}

// AssignTickets assigns unassigned tickets to agents based on keyword
// matching and priority. Already-assigned tickets are skipped. Tickets
// are processed in priority-descending order.
func (e *Engine) AssignTickets(tickets []sprintboard.Ticket) []Assignment {
	if len(tickets) == 0 {
		return nil
	}

	unassigned := make([]sprintboard.Ticket, 0, len(tickets))
	for _, t := range tickets {
		if t.OwnerAgent == "" {
			unassigned = append(unassigned, t)
		}
	}

	sort.Slice(unassigned, func(i, j int) bool {
		return unassigned[i].Priority > unassigned[j].Priority
	})

	assignments := make([]Assignment, 0, len(unassigned))
	for _, t := range unassigned {
		agentID := e.MatchAgent(t)
		assignments = append(assignments, Assignment{
			TicketID:    t.ID,
			AgentID:     agentID,
			TicketTitle: t.Title,
			TicketDesc:  t.Description,
			Priority:    t.Priority,
			Reason:      fmt.Sprintf("keyword match for %s", agentID),
		})
	}
	return assignments
}

// MatchAgent returns the best-fit agent ID for a ticket using keyword
// matching against ticket title and description. Falls back to
// cursor-parent as the default coordinator.
func (e *Engine) MatchAgent(t sprintboard.Ticket) string {
	text := strings.ToLower(t.Title + " " + t.Description)

	type scored struct {
		id    string
		score int
	}
	var scores []scored
	for id, profile := range e.profiles {
		s := 0
		for _, kw := range profile.Keywords {
			if strings.Contains(text, kw) {
				s++
			}
		}
		scores = append(scores, scored{id: id, score: s})
	}

	sort.Slice(scores, func(i, j int) bool {
		if scores[i].score != scores[j].score {
			return scores[i].score > scores[j].score
		}
		return scores[i].id < scores[j].id
	})

	if len(scores) > 0 && scores[0].score > 0 {
		return scores[0].id
	}
	return "cursor-parent"
}

// GenerateHandoffDoc produces a Markdown handoff document for an assignment.
func GenerateHandoffDoc(a Assignment) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Handoff: %s\n\n", a.TicketID))
	b.WriteString(fmt.Sprintf("**Assigned to**: %s\n", a.AgentID))
	b.WriteString(fmt.Sprintf("**Priority**: %d\n", a.Priority))
	b.WriteString(fmt.Sprintf("**Generated**: %s\n\n", time.Now().Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("## Task: %s\n\n", a.TicketTitle))
	if a.TicketDesc != "" {
		b.WriteString(a.TicketDesc + "\n\n")
	}
	b.WriteString("## Acceptance Criteria\n\n")
	b.WriteString("- [ ] Implementation complete\n")
	b.WriteString("- [ ] Tests pass (`go test -race ./...`)\n")
	b.WriteString("- [ ] No regressions\n\n")
	b.WriteString("## Context\n\n")
	b.WriteString("This handoff was auto-generated by `cursor-tools sprint-fanout`.\n")
	b.WriteString("Review the sprintboard ticket for full context.\n")
	return b.String()
}

// GenerateKickoffPrompt produces a prompt string that an agent can use
// to begin working on an assignment. Optionally includes recent Mem0
// memories for context.
func GenerateKickoffPrompt(a Assignment, recentMemories []string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("You are %s. You have been assigned ticket %s.\n\n", a.AgentID, a.TicketID))
	b.WriteString(fmt.Sprintf("## Task: %s\n\n", a.TicketTitle))
	if a.TicketDesc != "" {
		b.WriteString(a.TicketDesc + "\n\n")
	}
	b.WriteString(fmt.Sprintf("Priority: %d\n\n", a.Priority))

	if len(recentMemories) > 0 {
		b.WriteString("## Recent context from shared memory\n\n")
		for _, m := range recentMemories {
			b.WriteString(fmt.Sprintf("- %s\n", m))
		}
		b.WriteString("\n")
	}

	b.WriteString("## Rules\n\n")
	b.WriteString("- Follow TDD: write tests before implementation\n")
	b.WriteString("- Run `go test -race ./...` before marking complete\n")
	b.WriteString("- Do NOT commit or push -- prepare changes only\n")
	b.WriteString("- Read existing code before writing new code\n")
	return b.String()
}
