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
	"github.com/nfsarch33/cursor-tools/internal/outcomes"
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
	matcher        *patterns.Matcher
	log            *logger.Logger
	metricsPath    string
	outcomeEmitter outcomes.Emitter
}

func newGuardShellHandler() (*guardShellHandler, error) {
	m, err := patterns.NewMatcher(patterns.ShellDenyPatterns, patterns.ShellWarnPatterns)
	if err != nil {
		return nil, fmt.Errorf("compile patterns: %w", err)
	}
	paths := config.DefaultPaths()
	return &guardShellHandler{
		matcher:        m,
		log:            logger.New(paths.LogFile("guard-shell")),
		metricsPath:    paths.MetricsFile(),
		outcomeEmitter: hookOutcomeEmitter(paths),
	}, nil
}

func (h *guardShellHandler) Handle(_ context.Context, input *hookio.Input) (*hookio.Response, error) {
	start := time.Now()
	if input.Command == "" {
		return hookio.Allow(), nil
	}

	if d := identityStrictShellDeny(input.Command); d != nil {
		cmdShort := input.Command
		if len(cmdShort) > 120 {
			cmdShort = cmdShort[:120]
		}
		latencyMs := time.Since(start).Milliseconds()
		_ = metrics.Record(h.metricsPath, metrics.Event{
			Hook:      "guard-shell",
			Action:    "deny",
			Category:  "shell",
			LatencyMs: latencyMs,
			Detail:    "identity-strict: " + cmdShort,
			BytesIn:   int64(len(input.Command)),
		})
		recordHookOutcome(h.outcomeEmitter, hookOutcomeParams{
			hookName:  "guard-shell",
			action:    "deny",
			category:  "shell",
			latencyMs: latencyMs,
			detail:    "identity-strict: " + cmdShort,
			bytesIn:   int64(len(input.Command)),
			extraMeta: map[string]string{"reason": "identity-strict-gate"},
		})
		return d, nil
	}

	if d := strictFleetPreflightDeny(); d != nil {
		cmdShort := input.Command
		if len(cmdShort) > 120 {
			cmdShort = cmdShort[:120]
		}
		latencyMs := time.Since(start).Milliseconds()
		_ = metrics.Record(h.metricsPath, metrics.Event{
			Hook:      "guard-shell",
			Action:    "deny",
			Category:  "shell",
			LatencyMs: latencyMs,
			Detail:    "fleet-preflight-strict: " + cmdShort,
			BytesIn:   int64(len(input.Command)),
		})
		recordHookOutcome(h.outcomeEmitter, hookOutcomeParams{
			hookName:  "guard-shell",
			action:    "deny",
			category:  "shell",
			latencyMs: latencyMs,
			detail:    "fleet-preflight-strict: " + cmdShort,
			bytesIn:   int64(len(input.Command)),
			extraMeta: map[string]string{"reason": "fleet-preflight-strict"},
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

	latencyMs := time.Since(start).Milliseconds()
	_ = metrics.Record(h.metricsPath, metrics.Event{
		Hook:      "guard-shell",
		Action:    actionStr,
		Category:  "shell",
		LatencyMs: latencyMs,
		Detail:    cmdShort,
		BytesIn:   int64(len(input.Command)),
	})

	outcomeMeta := map[string]string{}
	if patternShort != "" {
		outcomeMeta["pattern"] = patternShort
	}
	recordHookOutcome(h.outcomeEmitter, hookOutcomeParams{
		hookName:  "guard-shell",
		action:    actionStr,
		category:  "shell",
		latencyMs: latencyMs,
		detail:    cmdShort,
		bytesIn:   int64(len(input.Command)),
		extraMeta: outcomeMeta,
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
