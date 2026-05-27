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

	pollSec := 30
	if v := os.Getenv("POLL_INTERVAL_SECONDS"); v != "" {
		if n, err := time.ParseDuration(v + "s"); err == nil {
			pollSec = int(n.Seconds())
		}
	}
	systemPrompt := loadPromptFile(envOrDefault("FLEET_SYSTEM_PROMPT", ""))
	cfg := fleetagent.Config{
		AgentID:      envOrDefault("FLEET_AGENT_ID", "fleet-agent-1"),
		Capabilities: []string{"go-build", "go-test", "docker", "k3s-deploy"},
		PollInterval: time.Duration(pollSec) * time.Second,
		MaxRetries:   3,
		SystemPrompt: systemPrompt,
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

func loadPromptFile(path string) string {
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
