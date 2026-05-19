package agentcapacity

import (
	"fmt"
	"sync"
	"time"
)

type AgentLoad struct {
	AgentID      string    `json:"agent_id"`
	Surface      string    `json:"surface"`
	ActiveTasks  int       `json:"active_tasks"`
	MaxCapacity  int       `json:"max_capacity"`
	LastActivity time.Time `json:"last_activity"`
}

type Planner struct {
	mu     sync.RWMutex
	agents map[string]*AgentLoad
}

func NewPlanner() *Planner {
	return &Planner{agents: make(map[string]*AgentLoad)}
}

func (p *Planner) Register(id, surface string, maxCapacity int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.agents[id] = &AgentLoad{
		AgentID:      id,
		Surface:      surface,
		MaxCapacity:  maxCapacity,
		LastActivity: time.Now(),
	}
}

func (p *Planner) Claim(agentID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	a, ok := p.agents[agentID]
	if !ok {
		return fmt.Errorf("agent %q not registered", agentID)
	}
	if a.ActiveTasks >= a.MaxCapacity {
		return fmt.Errorf("agent %q at capacity (%d/%d)", agentID, a.ActiveTasks, a.MaxCapacity)
	}
	a.ActiveTasks++
	a.LastActivity = time.Now()
	return nil
}

func (p *Planner) Release(agentID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	a, ok := p.agents[agentID]
	if !ok {
		return fmt.Errorf("agent %q not registered", agentID)
	}
	if a.ActiveTasks <= 0 {
		return fmt.Errorf("agent %q has no active tasks", agentID)
	}
	a.ActiveTasks--
	a.LastActivity = time.Now()
	return nil
}

func (p *Planner) Available() []AgentLoad {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var result []AgentLoad
	for _, a := range p.agents {
		if a.ActiveTasks < a.MaxCapacity {
			result = append(result, *a)
		}
	}
	return result
}

func (p *Planner) LeastLoaded() (string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var bestID string
	bestRatio := 2.0
	for _, a := range p.agents {
		if a.MaxCapacity == 0 {
			continue
		}
		ratio := float64(a.ActiveTasks) / float64(a.MaxCapacity)
		if ratio < bestRatio {
			bestRatio = ratio
			bestID = a.AgentID
		}
	}
	if bestID == "" {
		return "", fmt.Errorf("no agents available")
	}
	return bestID, nil
}

func (p *Planner) Stale(threshold time.Duration) []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	cutoff := time.Now().Add(-threshold)
	var stale []string
	for _, a := range p.agents {
		if a.LastActivity.Before(cutoff) {
			stale = append(stale, a.AgentID)
		}
	}
	return stale
}

func (p *Planner) Utilization() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var totalActive, totalCapacity int
	for _, a := range p.agents {
		totalActive += a.ActiveTasks
		totalCapacity += a.MaxCapacity
	}
	if totalCapacity == 0 {
		return 0
	}
	return float64(totalActive) / float64(totalCapacity)
}
