package fleetagent

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Config holds runtime configuration for the fleet agent.
type Config struct {
	AgentID      string
	Capabilities []string
	PollInterval time.Duration
	MaxRetries   int
	SystemPrompt string
}

// Agent implements the claim/execute/report loop.
type Agent struct {
	cfg    Config
	board  SprintBoardClient
	llm    LLMClient
	report Reporter
	log    *slog.Logger
}

// New creates a fleet agent with the given dependencies.
func New(cfg Config, board SprintBoardClient, llm LLMClient, reporter Reporter, log *slog.Logger) *Agent {
	if log == nil {
		log = slog.Default()
	}
	return &Agent{cfg: cfg, board: board, llm: llm, report: reporter, log: log}
}

// Run starts the polling loop. It blocks until ctx is cancelled.
func (a *Agent) Run(ctx context.Context) error {
	a.log.Info("fleet-agent starting", "agent_id", a.cfg.AgentID, "poll_interval", a.cfg.PollInterval)
	ticker := time.NewTicker(a.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			a.log.Info("fleet-agent shutting down")
			return ctx.Err()
		case <-ticker.C:
			if err := a.poll(ctx); err != nil {
				a.log.Error("poll cycle failed", "error", err)
			}
		}
	}
}

// RunOnce executes a single claim/execute/report cycle (useful for testing).
func (a *Agent) RunOnce(ctx context.Context) error {
	return a.poll(ctx)
}

func (a *Agent) poll(ctx context.Context) error {
	tickets, err := a.board.ListReady(ctx, a.cfg.Capabilities)
	if err != nil {
		return fmt.Errorf("list ready: %w", err)
	}
	if len(tickets) == 0 {
		return nil
	}

	ticket := tickets[0]
	result, err := a.board.Claim(ctx, ticket.ID, a.cfg.AgentID)
	if err != nil {
		return fmt.Errorf("claim %s: %w", ticket.ID, err)
	}
	if !result.Success {
		a.log.Info("claim conflict", "ticket", ticket.ID, "held_by", result.ConflictBy)
		return nil
	}

	a.log.Info("claimed ticket", "ticket", ticket.ID, "title", ticket.Title)
	execResult := a.execute(ctx, ticket)

	if err := a.report.Report(ctx, execResult); err != nil {
		a.log.Error("report failed", "ticket", ticket.ID, "error", err)
	}

	if execResult.Success {
		if err := a.board.Complete(ctx, ticket.ID, a.cfg.AgentID, execResult.Output); err != nil {
			return fmt.Errorf("complete %s: %w", ticket.ID, err)
		}
	} else {
		if err := a.board.Block(ctx, ticket.ID, a.cfg.AgentID, execResult.Error); err != nil {
			return fmt.Errorf("block %s: %w", ticket.ID, err)
		}
	}
	return nil
}

func (a *Agent) execute(ctx context.Context, t Ticket) ExecutionResult {
	start := time.Now()

	systemPrompt := a.cfg.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = fmt.Sprintf("You are fleet agent %q. Execute the following task and return a concise result.", a.cfg.AgentID)
	}
	userPrompt := fmt.Sprintf("Ticket: %s\nTitle: %s\nDescription: %s", t.ID, t.Title, t.Description)

	output, err := a.llm.Complete(ctx, systemPrompt, userPrompt)
	duration := time.Since(start)

	if err != nil {
		return ExecutionResult{
			TicketID:  t.ID,
			Success:   false,
			Error:     err.Error(),
			Duration:  duration,
			Timestamp: start,
		}
	}

	return ExecutionResult{
		TicketID:  t.ID,
		Success:   true,
		Output:    output,
		Duration:  duration,
		Timestamp: start,
	}
}
