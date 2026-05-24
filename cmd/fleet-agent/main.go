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
		AgentID:      envOrDefault("FLEET_AGENT_ID", "wsl1-fleet-agent"),
		Capabilities: []string{"go-build", "go-test", "docker", "k3s-deploy"},
		PollInterval: 30 * time.Second,
		MaxRetries:   3,
	}

	sprintboardURL := envOrDefault("SPRINTBOARD_URL", "http://127.0.0.1:9400")
	llmURL := envOrDefault("LLM_ROUTER_URL", "http://127.0.0.1:8787")
	llmModel := envOrDefault("LLM_MODEL", "Qwen/Qwen3.5-4B")
	engramURL := envOrDefault("ENGRAM_URL", "http://127.0.0.1:8281")

	board := fleetagent.NewHTTPSprintBoardClient(sprintboardURL)
	llm := fleetagent.NewHTTPLLMClient(llmURL, llmModel, "")
	reporter := fleetagent.NewEngramReporter(engramURL, "nfsarch33", "fleet-agent", "")

	log.Info("fleet-agent starting",
		"agent_id", cfg.AgentID,
		"capabilities", cfg.Capabilities,
		"sprintboard", sprintboardURL,
		"llm", llmURL,
		"engram", engramURL,
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	agent := fleetagent.New(cfg, board, llm, reporter, log)
	if err := agent.Run(ctx); err != nil && err != context.Canceled {
		log.Error("agent terminated", "error", err)
		os.Exit(1)
	}
	log.Info("fleet-agent stopped cleanly")
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
