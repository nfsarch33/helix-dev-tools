package workspace

import (
	"strings"
	"testing"
	"time"
)

func TestCoverageSummarisesWorkspaceAndHookEvents(t *testing.T) {
	now := time.Date(2026, 5, 6, 20, 0, 0, 0, time.UTC)
	workspaceJSONL := strings.NewReader(`{"generated_at":"2026-05-06T19:00:00Z","score":100,"tier":"GREEN","findings":0}
{"generated_at":"2026-05-06T19:10:00Z","score":57,"tier":"RED","findings":7}
{"generated_at":"2026-05-06T19:20:00Z","score":75,"tier":"YELLOW","findings":1}
`)
	metricsJSONL := strings.NewReader(`{"ts":"2026-05-06T19:00:00Z","hook":"guard-shell","cat":"shell","detail":"git commit -m x"}
{"ts":"2026-05-06T19:00:01Z","hook":"post-shell","cat":"workspace","detail":"workspace doctor"}
{"ts":"2026-05-06T19:01:00Z","hook":"guard-shell","cat":"shell","detail":"runx workspace doctor --quick"}
`)

	summary, err := SummariseCoverage(CoverageInput{
		WorkspaceEvents: workspaceJSONL,
		MetricsEvents:   metricsJSONL,
		Since:           now.Add(-24 * time.Hour),
		Now:             now,
	})
	if err != nil {
		t.Fatalf("SummariseCoverage: %v", err)
	}
	if summary.WorkspaceRuns != 3 {
		t.Fatalf("WorkspaceRuns = %d, want 3", summary.WorkspaceRuns)
	}
	if summary.RedCount != 1 || summary.YellowCount != 1 || summary.GreenCount != 1 {
		t.Fatalf("tier counts = green:%d yellow:%d red:%d", summary.GreenCount, summary.YellowCount, summary.RedCount)
	}
	if summary.GitMutationEvents != 1 || summary.PostShellEvents != 1 {
		t.Fatalf("events = git:%d post:%d", summary.GitMutationEvents, summary.PostShellEvents)
	}
	if summary.HookHitRate != 100 {
		t.Fatalf("HookHitRate = %.1f, want 100", summary.HookHitRate)
	}
}
