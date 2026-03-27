package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/cursor-tools/internal/config"
	"github.com/nfsarch33/cursor-tools/internal/hookio"
	"github.com/nfsarch33/cursor-tools/internal/logger"
	"github.com/nfsarch33/cursor-tools/internal/metrics"
	"github.com/nfsarch33/cursor-tools/internal/patterns"
)

var guardShellExit = os.Exit

var guardShellCmd = &cobra.Command{
	Use:   "guard-shell",
	Short: "beforeShellExecution: block dangerous shell commands",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGuardShell(os.Stdin, os.Stdout)
	},
}

type guardShellHandler struct {
	matcher     *patterns.Matcher
	log         *logger.Logger
	metricsPath string
}

func newGuardShellHandler() (*guardShellHandler, error) {
	m, err := patterns.NewMatcher(patterns.ShellDenyPatterns, patterns.ShellWarnPatterns)
	if err != nil {
		return nil, fmt.Errorf("compile patterns: %w", err)
	}
	paths := config.DefaultPaths()
	return &guardShellHandler{
		matcher:     m,
		log:         logger.New(paths.LogFile("guard-shell")),
		metricsPath: paths.MetricsFile(),
	}, nil
}

func (h *guardShellHandler) Handle(_ context.Context, input *hookio.Input) (*hookio.Response, error) {
	start := time.Now()
	if input.Command == "" {
		return hookio.Allow(), nil
	}

	if d := strictFleetPreflightDeny(); d != nil {
		cmdShort := input.Command
		if len(cmdShort) > 120 {
			cmdShort = cmdShort[:120]
		}
		_ = metrics.Record(h.metricsPath, metrics.Event{
			Hook:      "guard-shell",
			Action:    "deny",
			Category:  "shell",
			LatencyMs: time.Since(start).Milliseconds(),
			Detail:    "fleet-preflight-strict: " + cmdShort,
			BytesIn:   int64(len(input.Command)),
		})
		return d, nil
	}

	action, matchedPattern := h.matcher.Match(input.Command)

	cmdShort := input.Command
	if len(cmdShort) > 120 {
		cmdShort = cmdShort[:120]
	}
	patternShort := matchedPattern
	if len(patternShort) > 30 {
		patternShort = patternShort[:30]
	}

	var actionStr string
	var resp *hookio.Response

	switch action {
	case patterns.ActionDeny:
		actionStr = "deny"
		h.log.LogEntry(logger.Entry{
			Level:   "warn",
			Message: "shell command blocked",
			Hook:    "guard-shell",
			Result:  "deny",
			Fields: map[string]any{
				"command": cmdShort,
				"pattern": patternShort,
			},
		})
		resp = hookio.Deny(
			fmt.Sprintf("BLOCKED: dangerous command detected (pattern: %s...)", patternShort),
			"This command was BLOCKED by guard-shell because it matched a dangerous pattern. Do NOT attempt workarounds. Use a safe alternative.",
		)
	case patterns.ActionWarn:
		actionStr = "warn"
		h.log.LogEntry(logger.Entry{
			Level:   "warn",
			Message: "shell command requires confirmation",
			Hook:    "guard-shell",
			Result:  "warn",
			Fields: map[string]any{
				"command": cmdShort,
				"pattern": matchedPattern,
			},
		})
		cmdDisplay := input.Command
		if len(cmdDisplay) > 80 {
			cmdDisplay = cmdDisplay[:80]
		}
		resp = hookio.Ask(
			fmt.Sprintf("Requires confirmation: %s", cmdDisplay),
			"This command requires user confirmation. Ask the user before proceeding.",
		)
	default:
		actionStr = "allow"
		resp = hookio.Allow()
	}

	_ = metrics.Record(h.metricsPath, metrics.Event{
		Hook:      "guard-shell",
		Action:    actionStr,
		Category:  "shell",
		LatencyMs: time.Since(start).Milliseconds(),
		Detail:    cmdShort,
		BytesIn:   int64(len(input.Command)),
	})

	return resp, nil
}

func runGuardShell(stdin *os.File, stdout *os.File) error {
	handler, err := newGuardShellHandler()
	if err != nil {
		_ = hookio.WriteResponse(stdout, hookio.Allow())
		return nil
	}

	input, err := hookio.ReadInput(stdin)
	if err != nil {
		_ = hookio.WriteResponse(stdout, hookio.Allow())
		return nil
	}

	resp, err := handler.Handle(context.Background(), input)
	if err != nil {
		_ = hookio.WriteResponse(stdout, hookio.Allow())
		return nil
	}

	_ = hookio.WriteResponse(stdout, resp)
	if resp.Permission == "deny" {
		guardShellExit(2)
	}
	return nil
}
