package cli

import (
	"time"

	"github.com/nfsarch33/cursor-tools/internal/clilog"
	"github.com/nfsarch33/cursor-tools/internal/config"
	"github.com/nfsarch33/cursor-tools/internal/health"
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
	p := config.DefaultPaths()
	action := "pass"
	if pass < total {
		action = "fail"
	}
	_ = metrics.Record(p.MetricsFile(), metrics.Event{
		Hook:        name,
		Action:      action,
		Category:    "check",
		Detail:      name,
		DurationMs:  time.Since(started).Milliseconds(),
		LatencyMs:   time.Since(started).Milliseconds(),
		PassedCount: pass,
		TotalCount:  total,
	})
}
