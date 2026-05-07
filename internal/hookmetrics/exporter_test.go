package hookmetrics

import (
	"strings"
	"testing"
	"time"

	"github.com/nfsarch33/cursor-tools/internal/metrics"
)

func TestExporter_HitRateAndMutationLabels(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	events := []metrics.Event{
		{Timestamp: now.Add(-time.Hour), Category: "git", Action: "mutation", Detail: "runx git commit"},
		{Timestamp: now.Add(-time.Hour), Category: "git", Action: "mutation", Detail: "runx git push"},
		{Timestamp: now.Add(-time.Hour), Hook: "pre-push", Action: "allow"},
		{Timestamp: now.Add(-time.Hour), Hook: "post-shell", Action: "allow"},
		{Timestamp: now.Add(-48 * time.Hour), Category: "git", Action: "mutation", Detail: "old"},
	}

	out := ExportPrometheus(events, now.Add(-24*time.Hour))

	for _, want := range []string{
		`cursor_hook_git_mutations_total 2`,
		`cursor_hook_fires_total{hook="pre-push"} 1`,
		`cursor_hook_fires_total{hook="post-shell"} 1`,
		`cursor_hook_hit_rate 1`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("export missing %q:\n%s", want, out)
		}
	}
}
