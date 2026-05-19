package agentmatch

import (
	"sort"
	"strings"
)

type Agent struct {
	ID           string
	Capabilities []string
	Surface      string
	ActiveSince  int64
}

type Ticket struct {
	ID       string
	Title    string
	Tags     []string
	Priority int
}

type Match struct {
	AgentID  string
	TicketID string
	Score    float64
	Reason   string
}

func MatchAgentToTicket(agent Agent, ticket Ticket) float64 {
	if len(agent.Capabilities) == 0 || len(ticket.Tags) == 0 {
		return 0
	}

	capSet := make(map[string]bool)
	for _, c := range agent.Capabilities {
		capSet[strings.ToLower(c)] = true
	}

	matched := 0
	for _, tag := range ticket.Tags {
		if capSet[strings.ToLower(tag)] {
			matched++
		}
	}

	if len(ticket.Tags) == 0 {
		return 0
	}
	return float64(matched) / float64(len(ticket.Tags))
}

func RankAgentsForTicket(agents []Agent, ticket Ticket) []Match {
	var matches []Match
	for _, agent := range agents {
		score := MatchAgentToTicket(agent, ticket)
		if score > 0 {
			matches = append(matches, Match{
				AgentID:  agent.ID,
				TicketID: ticket.ID,
				Score:    score,
				Reason:   "capability match",
			})
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})
	return matches
}

func BestAgent(agents []Agent, ticket Ticket) (Agent, bool) {
	ranked := RankAgentsForTicket(agents, ticket)
	if len(ranked) == 0 {
		return Agent{}, false
	}
	for _, a := range agents {
		if a.ID == ranked[0].AgentID {
			return a, true
		}
	}
	return Agent{}, false
}

func DistributeTickets(agents []Agent, tickets []Ticket) map[string][]string {
	distribution := make(map[string][]string)
	assigned := make(map[string]bool)

	sorted := make([]Ticket, len(tickets))
	copy(sorted, tickets)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority > sorted[j].Priority
	})

	for _, ticket := range sorted {
		if assigned[ticket.ID] {
			continue
		}
		best, found := BestAgent(agents, ticket)
		if found {
			distribution[best.ID] = append(distribution[best.ID], ticket.ID)
			assigned[ticket.ID] = true
		}
	}
	return distribution
}
