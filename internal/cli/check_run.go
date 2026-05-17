package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nfsarch33/helix-dev-tools/internal/clilog"
	"github.com/nfsarch33/helix-dev-tools/internal/config"
	"github.com/nfsarch33/helix-dev-tools/internal/health"
	"github.com/nfsarch33/helix-dev-tools/internal/logger"
	"github.com/nfsarch33/helix-dev-tools/internal/metrics"
)

type checkReport struct {
	Title   string             `json:"title"`
	Command string             `json:"command"`
	Profile string             `json:"profile,omitempty"`
	RunID   string             `json:"run_id,omitempty"`
	Status  string             `json:"status"`
	Passed  int                `json:"passed"`
	Total   int                `json:"total"`
	Suites  []checkSuiteReport `json:"suites"`
}

type checkSuiteReport struct {
	Name       string              `json:"name"`
	Status     string              `json:"status"`
	Passed     int                 `json:"passed"`
	Total      int                 `json:"total"`
	DurationMs int64               `json:"duration_ms,omitempty"`
	Results    []checkResultReport `json:"results"`
}

type checkResultReport struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail,omitempty"`
}

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

func summarizeSuites(suites []*health.Suite) (int, int) {
	totalPass := 0
	totalCount := 0
	for _, suite := range suites {
		if suite == nil {
			continue
		}
		totalPass += suite.PassCount()
		totalCount += suite.Total()
	}
	return totalPass, totalCount
}

func buildCheckReportJSON(title, command, profile, runID string, suites []*health.Suite) ([]byte, error) {
	totalPass, totalCount := summarizeSuites(suites)
	report := checkReport{
		Title:   title,
		Command: command,
		Profile: profile,
		RunID:   runID,
		Status:  "pass",
		Passed:  totalPass,
		Total:   totalCount,
		Suites:  make([]checkSuiteReport, 0, len(suites)),
	}
	if totalPass < totalCount {
		report.Status = "fail"
	}
	for _, suite := range suites {
		if suite == nil {
			continue
		}
		suiteReport := checkSuiteReport{
			Name:       suite.Name,
			Status:     "pass",
			Passed:     suite.PassCount(),
			Total:      suite.Total(),
			DurationMs: suite.DurationMs,
			Results:    make([]checkResultReport, 0, len(suite.Results)),
		}
		if suiteReport.Passed < suiteReport.Total {
			suiteReport.Status = "fail"
		}
		for _, result := range suite.Results {
			suiteReport.Results = append(suiteReport.Results, checkResultReport{
				Name:   result.Name,
				Passed: result.Passed,
				Detail: result.Detail,
			})
		}
		report.Suites = append(report.Suites, suiteReport)
	}
	return json.MarshalIndent(report, "", "  ")
}

func writeCheckJSON(title, command, profile, runID string, suites []*health.Suite) error {
	data, err := buildCheckReportJSON(title, command, profile, runID, suites)
	if err != nil {
		return err
	}
	if _, err := os.Stdout.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}
