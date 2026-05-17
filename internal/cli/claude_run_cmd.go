package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/helix-dev-tools/internal/claude"
	"github.com/nfsarch33/helix-dev-tools/internal/clilog"
)

var claudeRunFlags struct {
	model   string
	timeout int
}

var claudeRunCmd = &cobra.Command{
	Use:   "claude-run [prompt]",
	Short: "Run Claude CLI through the usage tracker",
	Long: `Execute a prompt via Claude CLI and record token usage to ~/.cursor/claude-usage/.

  cursor-tools claude-run "Summarize this file"
  cursor-tools claude-run --model sonnet "Explain the architecture"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runClaudeRun,
}

func init() {
	claudeRunCmd.Flags().StringVar(&claudeRunFlags.model, "model", "", "Model name for tracking (informational)")
	claudeRunCmd.Flags().IntVar(&claudeRunFlags.timeout, "timeout", 300, "Timeout in seconds")
}

func runClaudeRun(_ *cobra.Command, args []string) error {
	prompt := strings.Join(args, " ")

	var opts []claude.Option
	if claudeRunFlags.model != "" {
		opts = append(opts, claude.WithModel(claudeRunFlags.model))
	}
	if claudeRunFlags.timeout > 0 {
		opts = append(opts, claude.WithTimeout(
			time.Duration(claudeRunFlags.timeout)*time.Second))
	}

	clilog.Info("routing prompt through Claude tracker (%d bytes)", len(prompt))

	output, usage, err := claude.Run(context.Background(), prompt, opts...)

	fmt.Println(output)

	clilog.Info("backend=%s duration=%dms prompt=%dB output=%dB exit=%d",
		usage.Backend, usage.DurationMs, usage.PromptBytes, usage.OutputBytes, usage.ExitCode)

	if usage.InputTokens > 0 {
		clilog.Info("tokens: input=%d output=%d cache_read=%d cache_write=%d cost=$%.4f",
			usage.InputTokens, usage.OutputTokens, usage.CacheRead, usage.CacheWrite, usage.Cost)
	}

	if err != nil {
		return fmt.Errorf("claude-run: %w", err)
	}
	return nil
}
