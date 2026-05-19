package agentprotocol

import (
	"fmt"
	"sync"
)

type Config struct {
	MaxAgents       int
	ClaimTimeoutSec int
}

type Agent struct {
	ID           string
	Surface      string
	Capabilities []string
}

type Protocol struct {
	config Config
	mu     sync.Mutex
	agents map[string]Agent
	claims map[string]string
}

func New(cfg Config) *Protocol {
	if cfg.MaxAgents == 0 {
		cfg.MaxAgents = 5
	}
	return &Protocol{
		config: cfg,
		agents: make(map[string]Agent),
		claims: make(map[string]string),
	}
}

func (p *Protocol) Register(a Agent) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.agents) >= p.config.MaxAgents {
		return fmt.Errorf("max agents (%d) exceeded", p.config.MaxAgents)
	}
	p.agents[a.ID] = a
	return nil
}

func (p *Protocol) Agents() []Agent {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]Agent, 0, len(p.agents))
	for _, a := range p.agents {
		out = append(out, a)
	}
	return out
}

func (p *Protocol) Claim(ticketID, agentID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if owner, ok := p.claims[ticketID]; ok && owner != agentID {
		return fmt.Errorf("conflict: ticket %s already claimed by %s", ticketID, owner)
	}
	p.claims[ticketID] = agentID
	return nil
}

func (p *Protocol) Release(ticketID, agentID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if owner, ok := p.claims[ticketID]; ok && owner == agentID {
		delete(p.claims, ticketID)
	}
}

func (p *Protocol) RouteToCapable(capability string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, a := range p.agents {
		for _, cap := range a.Capabilities {
			if cap == capability {
				return a.ID
			}
		}
	}
	return ""
}
