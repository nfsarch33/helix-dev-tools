package fleetagent

import (
	"context"
	"time"
)

// Ticket represents a claimable work item from SprintBoard.
type Ticket struct {
	ID          string
	SprintID    string
	Title       string
	Description string
	Status      string
	ClaimedBy   string
}

// ClaimResult is returned when attempting to claim a ticket.
type ClaimResult struct {
	Success    bool
	TicketID   string
	ConflictBy string
}

// ExecutionResult captures the outcome of executing a ticket.
type ExecutionResult struct {
	TicketID  string
	Success   bool
	Output    string
	Error     string
	Duration  time.Duration
	Timestamp time.Time
}

// SprintBoardClient provides access to the SprintBoard task queue.
type SprintBoardClient interface {
	ListReady(ctx context.Context, capabilities []string) ([]Ticket, error)
	Claim(ctx context.Context, ticketID, agentID string) (ClaimResult, error)
	Complete(ctx context.Context, ticketID, agentID, evidence string) error
	Block(ctx context.Context, ticketID, agentID, reason string) error
}

// LLMClient sends prompts to the language model for task execution.
type LLMClient interface {
	Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// Reporter publishes execution results to the memory/observability layer.
type Reporter interface {
	Report(ctx context.Context, result ExecutionResult) error
}
