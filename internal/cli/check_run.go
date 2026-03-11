package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/nfsarch33/cursor-tools/internal/clilog"
	"github.com/nfsarch33/cursor-tools/internal/config"
	"github.com/nfsarch33/cursor-tools/internal/health"
	"github.com/nfsarch33/cursor-tools/internal/logger"
	"github.com/nfsarch33/cursor-tools/internal/metrics"
)

func runSuites(title string, suites []*health.Suite) (int, int) {
	clilog.Header(title)
	runner := health.NewRunner()
	for _, suite := range suites {
		runner.Add(suite)
	}
	return runner.Run()
}

func recordCheckRun(name string, started time.Time, pass, total int) {
	recordCheckRunWithContext(name, name, "", started, pass, total)
}

func recordCheckRunWithContext(name, command, profile string, started time.Time, pass, total int) string {
	p := config.DefaultPaths()
	action := "pass"
	if pass < total {
		action = "fail"
	}
	runID := checkRunID(name, started)
	duration := time.Since(started).Milliseconds()
	_ = metrics.Record(p.MetricsFile(), metrics.Event{
		Hook:        name,
		Action:      action,
		Category:    "check",
		Detail:      name,
		DurationMs:  duration,
		LatencyMs:   duration,
		PassedCount: pass,
		TotalCount:  total,
		RunID:       runID,
		Command:     command,
		Profile:     profile,
	})
	logger.New(p.LogFile("checks")).LogEntry(logger.Entry{
		Level:      levelForCheckAction(action),
		Message:    "check run completed",
		Hook:       "check",
		Command:    command,
		Profile:    profile,
		Result:     action,
		RunID:      runID,
		DurationMs: duration,
		Fields: map[string]any{
			"passed": pass,
			"total":  total,
		},
	})
	return runID
}

func recordCheckSuiteRuns(command, profile, runID string, suites []*health.Suite) {
	if strings.TrimSpace(runID) == "" {
		return
	}
	p := config.DefaultPaths()
	checkLog := logger.New(p.LogFile("checks"))
	for _, suite := range suites {
		if suite == nil {
			continue
		}
		action := "pass"
		pass := suite.PassCount()
		total := suite.Total()
		if pass < total {
			action = "fail"
		}
		_ = metrics.Record(p.MetricsFile(), metrics.Event{
			Hook:        "check-suite",
			Action:      action,
			Category:    "check",
			Detail:      fmt.Sprintf("%s:%s", command, suite.Name),
			DurationMs:  suite.DurationMs,
			LatencyMs:   suite.DurationMs,
			PassedCount: pass,
			TotalCount:  total,
			RunID:       runID,
			Command:     command,
			Profile:     profile,
			Suite:       suite.Name,
		})
		checkLog.LogEntry(logger.Entry{
			Level:      levelForCheckAction(action),
			Message:    "suite completed",
			Hook:       "check",
			Command:    command,
			Profile:    profile,
			Suite:      suite.Name,
			Result:     action,
			RunID:      runID,
			DurationMs: suite.DurationMs,
			Fields: map[string]any{
				"passed": pass,
				"total":  total,
			},
		})
	}
}

func checkRunID(name string, started time.Time) string {
	slug := strings.NewReplacer(" ", "-", "_", "-").Replace(strings.ToLower(strings.TrimSpace(name)))
	return fmt.Sprintf("%s-%d", slug, started.UTC().UnixNano())
}

func levelForCheckAction(action string) string {
	switch action {
	case "fail":
		return "error"
	case "warn":
		return "warn"
	default:
		return "info"
	}
}
