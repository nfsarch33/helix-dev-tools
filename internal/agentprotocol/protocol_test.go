package agentprotocol

import (
	"testing"
)

func TestNewProtocol(t *testing.T) {
	p := New(Config{MaxAgents: 5, ClaimTimeoutSec: 300})
	if p == nil {
		t.Fatal("expected non-nil protocol")
	}
}

func TestRegisterAgent(t *testing.T) {
	p := New(Config{MaxAgents: 3})
	err := p.Register(Agent{ID: "cursor-parent", Surface: "cursor", Capabilities: []string{"go-tdd"}})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	agents := p.Agents()
	if len(agents) != 1 {
		t.Fatalf("got %d agents, want 1", len(agents))
	}
}

func TestMaxAgentsEnforced(t *testing.T) {
	p := New(Config{MaxAgents: 2})
	p.Register(Agent{ID: "a1", Surface: "cursor"})
	p.Register(Agent{ID: "a2", Surface: "codex"})
	err := p.Register(Agent{ID: "a3", Surface: "claude-code"})
	if err == nil {
		t.Error("expected error when exceeding max agents")
	}
}

func TestClaimConflict(t *testing.T) {
	p := New(Config{MaxAgents: 5})
	p.Register(Agent{ID: "a1", Surface: "cursor"})
	p.Register(Agent{ID: "a2", Surface: "codex"})

	err := p.Claim("ticket-1", "a1")
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}
	err = p.Claim("ticket-1", "a2")
	if err == nil {
		t.Error("expected conflict error on second claim")
	}
}

func TestRelease(t *testing.T) {
	p := New(Config{MaxAgents: 5})
	p.Register(Agent{ID: "a1", Surface: "cursor"})
	p.Claim("ticket-1", "a1")
	p.Release("ticket-1", "a1")

	err := p.Claim("ticket-1", "a1")
	if err != nil {
		t.Fatalf("re-claim after release: %v", err)
	}
}

func TestRouteToCapable(t *testing.T) {
	p := New(Config{MaxAgents: 5})
	p.Register(Agent{ID: "cursor", Surface: "cursor", Capabilities: []string{"go-tdd", "infra"}})
	p.Register(Agent{ID: "codex", Surface: "codex", Capabilities: []string{"ec-product", "testing"}})

	agent := p.RouteToCapable("go-tdd")
	if agent != "cursor" {
		t.Errorf("got %q, want cursor for go-tdd capability", agent)
	}
	agent = p.RouteToCapable("ec-product")
	if agent != "codex" {
		t.Errorf("got %q, want codex for ec-product capability", agent)
	}
}
