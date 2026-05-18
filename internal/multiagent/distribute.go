package multiagent

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// AgentProfile describes a coding agent's identity and capabilities.
type AgentProfile struct {
	ID           string
	Surface      string
	Capabilities []string
	MaxLoad      int
}

// HasCapability checks if the agent has a specific capability.
func (p AgentProfile) HasCapability(cap string) bool {
	for _, c := range p.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

func (p AgentProfile) matchScore(required []string) int {
	score := 0
	for _, req := range required {
		if p.HasCapability(req) {
			score++
		}
	}
	return score
}

// Ticket represents a work item to be distributed.
type Ticket struct {
	ID           string
	RequiredCaps []string
	Priority     int
	ClaimedBy    string
}

// Distributor assigns tickets to agents based on capabilities and load.
type Distributor struct {
	agents []AgentProfile
}

// NewDistributor creates an empty distributor.
func NewDistributor() *Distributor {
	return &Distributor{}
}

// RegisterAgent adds an agent to the distribution pool.
func (d *Distributor) RegisterAgent(p AgentProfile) {
	d.agents = append(d.agents, p)
}

// Distribute assigns tickets to agents. Returns map of ticketID -> agentID.
func (d *Distributor) Distribute(tickets []Ticket) map[string]string {
	if len(d.agents) == 0 {
		return nil
	}

	assignments := make(map[string]string)
	load := make(map[string]int)

	sorted := make([]Ticket, len(tickets))
	copy(sorted, tickets)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority > sorted[j].Priority
	})

	for _, ticket := range sorted {
		bestAgent := ""
		bestScore := 0

		for _, agent := range d.agents {
			if load[agent.ID] >= agent.MaxLoad {
				continue
			}
			score := agent.matchScore(ticket.RequiredCaps)
			if score == 0 {
				continue
			}
			if score > bestScore || (score == bestScore && load[agent.ID] < load[bestAgent]) {
				bestAgent = agent.ID
				bestScore = score
			}
		}

		if bestAgent != "" {
			assignments[ticket.ID] = bestAgent
			load[bestAgent]++
		}
	}

	return assignments
}

// Recommender suggests next tasks for a specific agent.
type Recommender struct {
	agents  map[string]AgentProfile
	tickets []Ticket
}

// NewRecommender creates a recommender instance.
func NewRecommender() *Recommender {
	return &Recommender{
		agents: make(map[string]AgentProfile),
	}
}

// RegisterAgent adds an agent profile for recommendation matching.
func (r *Recommender) RegisterAgent(p AgentProfile) {
	r.agents[p.ID] = p
}

// AddTickets adds tickets to the recommendation pool.
func (r *Recommender) AddTickets(tickets []Ticket) {
	r.tickets = append(r.tickets, tickets...)
}

// Recommend returns up to limit tickets suitable for the given agent.
func (r *Recommender) Recommend(agentID string, limit int) []Ticket {
	agent, ok := r.agents[agentID]
	if !ok {
		return nil
	}

	type scored struct {
		ticket Ticket
		score  int
	}
	var candidates []scored

	for _, t := range r.tickets {
		if t.ClaimedBy != "" {
			continue
		}
		score := agent.matchScore(t.RequiredCaps)
		if score > 0 {
			candidates = append(candidates, scored{ticket: t, score: score})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].ticket.Priority != candidates[j].ticket.Priority {
			return candidates[i].ticket.Priority > candidates[j].ticket.Priority
		}
		return candidates[i].score > candidates[j].score
	})

	var result []Ticket
	for i := 0; i < len(candidates) && i < limit; i++ {
		result = append(result, candidates[i].ticket)
	}
	return result
}

// HandoffTemplate generates structured handoff documents.
type HandoffTemplate struct {
	FromAgent string
	ToAgent   string
	TicketID  string
	Summary   string
	Branch    string
	Evidence  string
}

// ToMarkdown renders the handoff as a markdown document.
func (h HandoffTemplate) ToMarkdown() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Handoff: %s -> %s\n\n", h.FromAgent, h.ToAgent))
	sb.WriteString(fmt.Sprintf("**Date**: %s\n", time.Now().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("**Ticket**: %s\n", h.TicketID))
	sb.WriteString(fmt.Sprintf("**Branch**: %s\n\n", h.Branch))
	sb.WriteString("## Summary\n\n")
	sb.WriteString(h.Summary + "\n\n")
	sb.WriteString("## Evidence\n\n")
	sb.WriteString(h.Evidence + "\n\n")
	sb.WriteString("## Next Steps\n\n")
	sb.WriteString(fmt.Sprintf("- [ ] %s picks up from branch `%s`\n", h.ToAgent, h.Branch))
	sb.WriteString("- [ ] Run tests and verify\n")
	sb.WriteString("- [ ] Push and open PR\n")
	return sb.String()
}
