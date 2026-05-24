package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nfsarch33/helix-dev-tools/internal/fleetagent"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg := fleetagent.Config{
		AgentID:      envOrDefault("FLEET_AGENT_ID", "fleet-agent-1"),
		Capabilities: []string{"go-build", "go-test", "docker"},
		PollInterval: 30 * time.Second,
		MaxRetries:   3,
	}

	// Placeholder implementations -- replaced with real clients at wire-up time.
	var board fleetagent.SprintBoardClient
	var llm fleetagent.LLMClient
	var reporter fleetagent.Reporter

	_ = board
	_ = llm
	_ = reporter

	log.Info("fleet-agent configured", "agent_id", cfg.AgentID, "capabilities", cfg.Capabilities)

	// Wire-up deferred until real SprintBoard HTTP client, MiniMax LLM client,
	// and Engram reporter are implemented. For now, exit cleanly.
	log.Warn("real client implementations not wired -- exiting")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	_ = ctx
	// Uncomment when clients are ready:
	// agent := fleetagent.New(cfg, board, llm, reporter, log)
	// if err := agent.Run(ctx); err != nil && err != context.Canceled {
	//     log.Error("agent terminated", "error", err)
	//     os.Exit(1)
	// }
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
