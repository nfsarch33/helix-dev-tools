package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/nfsarch33/helix-dev-tools/internal/sprintdispatch"
	"github.com/spf13/cobra"
)

var sprintDispatchFlags struct {
	agent    string
	kickoff  string
	sprintID string
	exec     bool
}

var sprintDispatchCmd = &cobra.Command{
	Use:   "sprint-dispatch",
	Short: "Build headless agent dispatch commands from a kickoff handoff",
	Long: `Prepares copy-paste-free dispatch for overnight agent sessions.

  cursor-tools sprint-dispatch --agent codex --kickoff ~/Code/global-kb/session-handoffs/<file>.md --sprint v7100
  cursor-tools sprint-dispatch --agent claude-code --kickoff <path> --sprint v7100

Codex: prints a shell one-liner with positional prompt, stdin closed (</dev/null), log under ~/logs/runx/.
Claude Code: prints a claude -p template (no secrets on argv beyond kickoff path).

Use --exec to launch codex in the background (stdin from /dev/null, stdout/stderr to log).`,
	RunE: runSprintDispatch,
}

func init() {
	sprintDispatchCmd.Flags().StringVar(&sprintDispatchFlags.agent, "agent", "", "Agent surface: claude-code or codex (required)")
	sprintDispatchCmd.Flags().StringVar(&sprintDispatchFlags.kickoff, "kickoff", "", "Path to kickoff handoff markdown (required)")
	sprintDispatchCmd.Flags().StringVar(&sprintDispatchFlags.sprintID, "sprint", "", "Sprint id, e.g. v7100 (required)")
	sprintDispatchCmd.Flags().BoolVar(&sprintDispatchFlags.exec, "exec", false, "Execute codex dispatch in background (codex agent only)")
}

func runSprintDispatch(cmd *cobra.Command, _ []string) error {
	agent, err := sprintdispatch.ValidateAgent(sprintDispatchFlags.agent)
	if err != nil {
		return err
	}

	result, err := sprintdispatch.Build(sprintdispatch.Spec{
		Agent:       agent,
		KickoffPath: sprintDispatchFlags.kickoff,
		SprintID:    sprintDispatchFlags.sprintID,
	})
	if err != nil {
		return err
	}

	fmt.Fprintln(cmd.OutOrStdout(), "# sprint-dispatch command (copy or use --exec for codex)")
	fmt.Fprintln(cmd.OutOrStdout(), result.Command)
	fmt.Fprintf(cmd.OutOrStdout(), "\nlog_path=%s\n", result.LogPath)
	fmt.Fprintf(cmd.OutOrStdout(), "kickoff=%s\n", result.KickoffPath)

	if err := appendDispatchNDJSON(result); err != nil {
		fmt.Fprintf(os.Stderr, "warning: dispatch ndjson: %v\n", err)
	}

	if sprintDispatchFlags.exec {
		if agent != sprintdispatch.AgentCodex {
			return fmt.Errorf("sprint-dispatch: --exec is only supported for --agent codex")
		}
		return execCodexDispatch(result)
	}
	return nil
}

func appendDispatchNDJSON(result sprintdispatch.Result) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	path := filepath.Join(home, "logs", "runx", "agent-dispatch.ndjson")
	ev := map[string]string{
		"event":    "sprint_dispatch",
		"ts":       time.Now().Format(time.RFC3339),
		"agent":    string(result.Agent),
		"sprint":   result.SprintID,
		"kickoff":  result.KickoffPath,
		"log_path": result.LogPath,
	}
	data, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(data, '\n'))
	return err
}

func execCodexDispatch(result sprintdispatch.Result) error {
	if len(result.ExecArgv) == 0 {
		return fmt.Errorf("sprint-dispatch: missing exec argv")
	}
	logFile, err := os.OpenFile(result.LogPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("sprint-dispatch: open log: %w", err)
	}
	defer logFile.Close()

	c := exec.Command(result.ExecArgv[0], result.ExecArgv[1:]...)
	c.Stdout = logFile
	c.Stderr = logFile
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		return fmt.Errorf("sprint-dispatch: open /dev/null: %w", err)
	}
	defer devNull.Close()
	c.Stdin = devNull

	if err := c.Start(); err != nil {
		return fmt.Errorf("sprint-dispatch: start codex: %w", err)
	}
	fmt.Fprintf(os.Stdout, "started codex pid=%d log=%s\n", c.Process.Pid, result.LogPath)
	return nil
}
