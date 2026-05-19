package agentmatch

import "testing"

func TestMatchAgentToTicket_FullMatch(t *testing.T) {
	agent := Agent{ID: "a1", Capabilities: []string{"go-tdd", "testing"}}
	ticket := Ticket{ID: "t1", Tags: []string{"go-tdd", "testing"}}

	score := MatchAgentToTicket(agent, ticket)
	if score != 1.0 {
		t.Errorf("expected 1.0 for full match, got %f", score)
	}
}

func TestMatchAgentToTicket_PartialMatch(t *testing.T) {
	agent := Agent{ID: "a1", Capabilities: []string{"go-tdd", "infra"}}
	ticket := Ticket{ID: "t1", Tags: []string{"go-tdd", "testing", "review"}}

	score := MatchAgentToTicket(agent, ticket)
	expected := 1.0 / 3.0
	if score < expected-0.01 || score > expected+0.01 {
		t.Errorf("expected ~0.33, got %f", score)
	}
}

func TestMatchAgentToTicket_NoMatch(t *testing.T) {
	agent := Agent{ID: "a1", Capabilities: []string{"flutter"}}
	ticket := Ticket{ID: "t1", Tags: []string{"go-tdd"}}

	score := MatchAgentToTicket(agent, ticket)
	if score != 0 {
		t.Errorf("expected 0 for no match, got %f", score)
	}
}

func TestMatchAgentToTicket_EmptyCapabilities(t *testing.T) {
	agent := Agent{ID: "a1", Capabilities: nil}
	ticket := Ticket{ID: "t1", Tags: []string{"go-tdd"}}

	score := MatchAgentToTicket(agent, ticket)
	if score != 0 {
		t.Error("expected 0 for empty capabilities")
	}
}

func TestRankAgentsForTicket(t *testing.T) {
	agents := []Agent{
		{ID: "cursor", Capabilities: []string{"go-tdd", "infra", "testing"}},
		{ID: "claude-code", Capabilities: []string{"go-tdd", "testing"}},
		{ID: "codex", Capabilities: []string{"review"}},
	}
	ticket := Ticket{ID: "t1", Tags: []string{"go-tdd", "testing"}}

	ranked := RankAgentsForTicket(agents, ticket)

	if len(ranked) < 2 {
		t.Fatalf("expected at least 2 matches, got %d", len(ranked))
	}
	if ranked[0].AgentID != "cursor" && ranked[0].AgentID != "claude-code" {
		t.Errorf("expected cursor or claude-code as top match, got %s", ranked[0].AgentID)
	}
}

func TestBestAgent_Found(t *testing.T) {
	agents := []Agent{
		{ID: "a", Capabilities: []string{"infra"}},
		{ID: "b", Capabilities: []string{"go-tdd", "testing"}},
	}
	ticket := Ticket{ID: "t1", Tags: []string{"go-tdd"}}

	best, found := BestAgent(agents, ticket)
	if !found {
		t.Fatal("expected to find best agent")
	}
	if best.ID != "b" {
		t.Errorf("expected agent b, got %s", best.ID)
	}
}

func TestBestAgent_NoneMatch(t *testing.T) {
	agents := []Agent{{ID: "a", Capabilities: []string{"flutter"}}}
	ticket := Ticket{ID: "t1", Tags: []string{"go-tdd"}}

	_, found := BestAgent(agents, ticket)
	if found {
		t.Error("expected no match")
	}
}

func TestDistributeTickets(t *testing.T) {
	agents := []Agent{
		{ID: "cursor", Capabilities: []string{"go-tdd", "infra"}},
		{ID: "claude-code", Capabilities: []string{"go-tdd", "testing"}},
	}
	tickets := []Ticket{
		{ID: "t1", Tags: []string{"infra"}, Priority: 9},
		{ID: "t2", Tags: []string{"testing"}, Priority: 8},
		{ID: "t3", Tags: []string{"go-tdd"}, Priority: 7},
	}

	dist := DistributeTickets(agents, tickets)

	if len(dist["cursor"]) == 0 {
		t.Error("expected cursor to get tickets")
	}
	if len(dist["claude-code"]) == 0 {
		t.Error("expected claude-code to get tickets")
	}
}

func TestMatchAgentToTicket_CaseInsensitive(t *testing.T) {
	agent := Agent{ID: "a1", Capabilities: []string{"Go-TDD"}}
	ticket := Ticket{ID: "t1", Tags: []string{"go-tdd"}}

	score := MatchAgentToTicket(agent, ticket)
	if score != 1.0 {
		t.Errorf("expected case-insensitive match, got %f", score)
	}
}
