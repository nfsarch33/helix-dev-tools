package sprintdispatch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxCodexPromptBytes = 8000

// Agent identifies the dispatch target surface.
type Agent string

const (
	AgentClaudeCode Agent = "claude-code"
	AgentCodex      Agent = "codex"
)

// Spec holds validated dispatch inputs.
type Spec struct {
	Agent       Agent
	KickoffPath string
	SprintID    string
}

// Result is the built dispatch artefact (command template or shell invocation).
type Result struct {
	Agent       Agent
	SprintID    string
	KickoffPath string
	LogPath     string
	// Command is a copy-pasteable shell fragment (no secrets).
	Command string
	// ExecArgv is set for codex when --exec is used: argv for codex binary.
	ExecArgv []string
}

// ValidateAgent normalizes and validates --agent.
func ValidateAgent(raw string) (Agent, error) {
	a := Agent(strings.TrimSpace(strings.ToLower(raw)))
	switch a {
	case AgentClaudeCode, AgentCodex:
		return a, nil
	default:
		return "", fmt.Errorf("sprint-dispatch: --agent must be claude-code or codex (got %q)", raw)
	}
}

// Build constructs the dispatch command for the given spec.
func Build(spec Spec) (Result, error) {
	if spec.SprintID == "" {
		return Result{}, fmt.Errorf("sprint-dispatch: --sprint is required")
	}
	kickoff, err := resolveKickoff(spec.KickoffPath)
	if err != nil {
		return Result{}, err
	}
	spec.KickoffPath = kickoff

	logPath := defaultLogPath(spec.Agent, spec.SprintID)

	switch spec.Agent {
	case AgentCodex:
		return buildCodex(spec, logPath)
	case AgentClaudeCode:
		return buildClaudeCode(spec, logPath), nil
	default:
		return Result{}, fmt.Errorf("sprint-dispatch: unsupported agent %q", spec.Agent)
	}
}

func resolveKickoff(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("sprint-dispatch: --kickoff path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("sprint-dispatch: kickoff path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("sprint-dispatch: kickoff file: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("sprint-dispatch: kickoff must be a file, not a directory")
	}
	return abs, nil
}

func defaultLogPath(agent Agent, sprintID string) string {
	home, _ := os.UserHomeDir()
	ts := time.Now().Format("20060102T150405")
	name := fmt.Sprintf("%s-dispatch-%s-%s.log", agent, sprintID, ts)
	return filepath.Join(home, "logs", "runx", name)
}

func buildCodex(spec Spec, logPath string) (Result, error) {
	prompt, err := readPromptPrefix(spec.KickoffPath, maxCodexPromptBytes)
	if err != nil {
		return Result{}, err
	}

	argv := []string{
		"codex",
		"exec",
		"--skip-git-repo-check",
		"--dangerously-bypass-approvals-and-sandbox",
		"--json",
		prompt,
	}

	shell := fmt.Sprintf(`KICKOFF=%q
LOG=%q
codex exec --skip-git-repo-check \
  --dangerously-bypass-approvals-and-sandbox \
  --json \
  "$(head -c %d < "$KICKOFF")" </dev/null \
  > "$LOG" 2>&1 &
echo "codex dispatch: sprint=%s log=$LOG pid=$!"`,
		spec.KickoffPath, logPath, maxCodexPromptBytes, spec.SprintID)

	return Result{
		Agent:       AgentCodex,
		SprintID:    spec.SprintID,
		KickoffPath: spec.KickoffPath,
		LogPath:     logPath,
		Command:     shell,
		ExecArgv:    argv,
	}, nil
}

func buildClaudeCode(spec Spec, logPath string) Result {
	prompt := fmt.Sprintf(
		"Read the kickoff handoff at %s and execute sprint %s using the Sprintboard MCP protocol (agent_register, handoff_subscribe, sprint_status, task_claim, task_complete, handoff_publish). Use semble search before rg/grep for discovery.",
		spec.KickoffPath,
		spec.SprintID,
	)

	shell := fmt.Sprintf(`KICKOFF=%q
LOG=%q
claude -p %q \
  --add-dir "$HOME/Code/global-kb" \
  > "$LOG" 2>&1 &
echo "claude-code dispatch: sprint=%s log=$LOG pid=$!"`,
		spec.KickoffPath, logPath, prompt, spec.SprintID)

	return Result{
		Agent:       AgentClaudeCode,
		SprintID:    spec.SprintID,
		KickoffPath: spec.KickoffPath,
		LogPath:     logPath,
		Command:     shell,
	}
}

func readPromptPrefix(path string, maxBytes int) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("sprint-dispatch: read kickoff: %w", err)
	}
	if len(data) > maxBytes {
		data = data[:maxBytes]
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return "", fmt.Errorf("sprint-dispatch: kickoff file is empty")
	}
	return text, nil
}
