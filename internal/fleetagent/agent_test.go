package fleetagent_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/nfsarch33/helix-dev-tools/internal/fleetagent"
)

// --- Mocks ---

type mockBoard struct {
	tickets     []fleetagent.Ticket
	claimResult fleetagent.ClaimResult
	claimErr    error
	completeErr error
	blockErr    error
	claimed     []string
	completed   []string
	blocked     []string
}

func (m *mockBoard) ListReady(_ context.Context, _ []string) ([]fleetagent.Ticket, error) {
	return m.tickets, nil
}

func (m *mockBoard) Claim(_ context.Context, ticketID, agentID string) (fleetagent.ClaimResult, error) {
	m.claimed = append(m.claimed, ticketID)
	return m.claimResult, m.claimErr
}

func (m *mockBoard) Complete(_ context.Context, ticketID, agentID, evidence string) error {
	m.completed = append(m.completed, ticketID)
	return m.completeErr
}

func (m *mockBoard) Block(_ context.Context, ticketID, agentID, reason string) error {
	m.blocked = append(m.blocked, ticketID)
	return m.blockErr
}

type mockLLM struct {
	response string
	err      error
}

func (m *mockLLM) Complete(_ context.Context, _, _ string) (string, error) {
	return m.response, m.err
}

type mockReporter struct {
	results []fleetagent.ExecutionResult
	err     error
}

func (m *mockReporter) Report(_ context.Context, r fleetagent.ExecutionResult) error {
	m.results = append(m.results, r)
	return m.err
}

// --- Tests ---

func TestAgent_RunOnce_NoTickets(t *testing.T) {
	board := &mockBoard{tickets: nil, claimResult: fleetagent.ClaimResult{Success: true}}
	llm := &mockLLM{response: "done"}
	reporter := &mockReporter{}

	agent := fleetagent.New(fleetagent.Config{
		AgentID:      "test-agent",
		Capabilities: []string{"go-test"},
		PollInterval: time.Second,
	}, board, llm, reporter, slog.Default())

	err := agent.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(board.claimed) != 0 {
		t.Errorf("expected no claims, got %d", len(board.claimed))
	}
}

func TestAgent_RunOnce_ClaimAndComplete(t *testing.T) {
	board := &mockBoard{
		tickets: []fleetagent.Ticket{
			{ID: "T-001", Title: "Build widget", Description: "Implement the widget"},
		},
		claimResult: fleetagent.ClaimResult{Success: true, TicketID: "T-001"},
	}
	llm := &mockLLM{response: "Widget built successfully"}
	reporter := &mockReporter{}

	agent := fleetagent.New(fleetagent.Config{
		AgentID:      "test-agent",
		Capabilities: []string{"go-build"},
		PollInterval: time.Second,
	}, board, llm, reporter, slog.Default())

	err := agent.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(board.claimed) != 1 || board.claimed[0] != "T-001" {
		t.Errorf("expected claim on T-001, got %v", board.claimed)
	}
	if len(board.completed) != 1 || board.completed[0] != "T-001" {
		t.Errorf("expected complete on T-001, got %v", board.completed)
	}
	if len(reporter.results) != 1 || !reporter.results[0].Success {
		t.Error("expected successful report")
	}
}

func TestAgent_RunOnce_ClaimConflict(t *testing.T) {
	board := &mockBoard{
		tickets: []fleetagent.Ticket{
			{ID: "T-002", Title: "Contested ticket"},
		},
		claimResult: fleetagent.ClaimResult{Success: false, ConflictBy: "other-agent"},
	}
	llm := &mockLLM{response: "should not be called"}
	reporter := &mockReporter{}

	agent := fleetagent.New(fleetagent.Config{
		AgentID:      "test-agent",
		Capabilities: []string{"go-test"},
		PollInterval: time.Second,
	}, board, llm, reporter, slog.Default())

	err := agent.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(board.completed) != 0 {
		t.Error("should not complete a conflicted ticket")
	}
	if len(reporter.results) != 0 {
		t.Error("should not report for conflicted claim")
	}
}

func TestAgent_RunOnce_LLMFailure_Blocks(t *testing.T) {
	board := &mockBoard{
		tickets: []fleetagent.Ticket{
			{ID: "T-003", Title: "Failing task"},
		},
		claimResult: fleetagent.ClaimResult{Success: true, TicketID: "T-003"},
	}
	llm := &mockLLM{err: errors.New("model timeout")}
	reporter := &mockReporter{}

	agent := fleetagent.New(fleetagent.Config{
		AgentID:      "test-agent",
		Capabilities: []string{"go-test"},
		PollInterval: time.Second,
	}, board, llm, reporter, slog.Default())

	err := agent.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(board.blocked) != 1 || board.blocked[0] != "T-003" {
		t.Errorf("expected block on T-003, got %v", board.blocked)
	}
	if len(reporter.results) != 1 || reporter.results[0].Success {
		t.Error("expected failed report")
	}
}

func TestAgent_RunOnce_ReportError_NonFatal(t *testing.T) {
	board := &mockBoard{
		tickets: []fleetagent.Ticket{
			{ID: "T-004", Title: "Report failure test"},
		},
		claimResult: fleetagent.ClaimResult{Success: true, TicketID: "T-004"},
	}
	llm := &mockLLM{response: "done"}
	reporter := &mockReporter{err: errors.New("engram unavailable")}

	agent := fleetagent.New(fleetagent.Config{
		AgentID:      "test-agent",
		Capabilities: []string{"go-test"},
		PollInterval: time.Second,
	}, board, llm, reporter, slog.Default())

	err := agent.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("report error should not propagate, got: %v", err)
	}
	if len(board.completed) != 1 {
		t.Error("ticket should still complete despite report failure")
	}
}

func TestAgent_Run_CancelledContext(t *testing.T) {
	board := &mockBoard{tickets: nil}
	llm := &mockLLM{}
	reporter := &mockReporter{}

	agent := fleetagent.New(fleetagent.Config{
		AgentID:      "test-agent",
		PollInterval: 50 * time.Millisecond,
	}, board, llm, reporter, slog.Default())

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()

	err := agent.Run(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
}
