package workdist

import (
	"testing"
)

func TestDistributeRoundRobin_EvenDistribution(t *testing.T) {
	tickets := []Ticket{
		{ID: "T-1"}, {ID: "T-2"}, {ID: "T-3"}, {ID: "T-4"},
	}
	agents := []Agent{
		{ID: "A"}, {ID: "B"},
	}

	dist := DistributeRoundRobin(tickets, agents)

	if len(dist["A"]) != 2 {
		t.Fatalf("expected agent A to get 2 tickets, got %d", len(dist["A"]))
	}
	if len(dist["B"]) != 2 {
		t.Fatalf("expected agent B to get 2 tickets, got %d", len(dist["B"]))
	}
}

func TestDistributeByCapability_Matching(t *testing.T) {
	tickets := []Ticket{
		{ID: "T-1", Capability: "go"},
		{ID: "T-2", Capability: "rust"},
		{ID: "T-3", Capability: "go"},
	}
	agents := []Agent{
		{ID: "A", Capabilities: []string{"go"}},
		{ID: "B", Capabilities: []string{"rust"}},
	}

	dist := DistributeByCapability(tickets, agents)

	if len(dist["A"]) != 2 {
		t.Fatalf("expected agent A to get 2 go tickets, got %d", len(dist["A"]))
	}
	if len(dist["B"]) != 1 {
		t.Fatalf("expected agent B to get 1 rust ticket, got %d", len(dist["B"]))
	}
}

func TestDistributeRoundRobin_EmptyInputs(t *testing.T) {
	dist := DistributeRoundRobin(nil, nil)
	if len(dist) != 0 {
		t.Fatal("expected empty map for nil agents")
	}

	dist2 := DistributeRoundRobin([]Ticket{{ID: "T-1"}}, nil)
	if len(dist2) != 0 {
		t.Fatal("expected empty map for nil agents with tickets")
	}
}

func TestDistributeByCapability_EmptyInputs(t *testing.T) {
	dist := DistributeByCapability(nil, []Agent{{ID: "A", Capabilities: []string{"go"}}})
	if len(dist["A"]) != 0 {
		t.Fatal("expected no tickets assigned")
	}
}
