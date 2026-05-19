package sprintboard

import (
	"github.com/nfsarch33/helix-dev-tools/internal/multiagent"
)

// RecommendTasks finds the best tickets for a given agent based on capabilities.
func RecommendTasks(agentID string, agents []multiagent.AgentProfile, tickets []multiagent.Ticket, limit int) []multiagent.Ticket {
	r := multiagent.NewRecommender()
	for _, a := range agents {
		r.RegisterAgent(a)
	}
	r.AddTickets(tickets)
	return r.Recommend(agentID, limit)
}

// DistributeTasks assigns all tickets across available agents optimally.
func DistributeTasks(agents []multiagent.AgentProfile, tickets []multiagent.Ticket) map[string]string {
	d := multiagent.NewDistributor()
	for _, a := range agents {
		d.RegisterAgent(a)
	}
	return d.Distribute(tickets)
}
