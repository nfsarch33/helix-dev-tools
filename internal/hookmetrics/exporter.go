package hookmetrics

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/nfsarch33/cursor-tools/internal/metrics"
)

// ExportPrometheus renders hook hit-rate metrics in Prometheus text format.
func ExportPrometheus(events []metrics.Event, since time.Time) string {
	hookCounts := map[string]int{}
	mutations := 0
	for _, event := range events {
		if event.Timestamp.Before(since) {
			continue
		}
		if isGitMutation(event) {
			mutations++
		}
		if event.Hook != "" {
			hookCounts[event.Hook]++
		}
	}
	hooks := make([]string, 0, len(hookCounts))
	for hook := range hookCounts {
		hooks = append(hooks, hook)
	}
	sort.Strings(hooks)
	var b strings.Builder
	fmt.Fprintln(&b, "# HELP cursor_hook_git_mutations_total Git mutation events observed by Cursor hooks.")
	fmt.Fprintln(&b, "# TYPE cursor_hook_git_mutations_total counter")
	fmt.Fprintf(&b, "cursor_hook_git_mutations_total %d\n", mutations)
	fmt.Fprintln(&b, "# HELP cursor_hook_fires_total Hook fire events by hook name.")
	fmt.Fprintln(&b, "# TYPE cursor_hook_fires_total counter")
	totalHooks := 0
	for _, hook := range hooks {
		totalHooks += hookCounts[hook]
		fmt.Fprintf(&b, "cursor_hook_fires_total{hook=%q} %d\n", hook, hookCounts[hook])
	}
	hitRate := 0.0
	if mutations > 0 {
		hitRate = float64(totalHooks) / float64(mutations)
		if hitRate > 1 {
			hitRate = 1
		}
	}
	fmt.Fprintln(&b, "# HELP cursor_hook_hit_rate Ratio of hook fires to git mutation events.")
	fmt.Fprintln(&b, "# TYPE cursor_hook_hit_rate gauge")
	fmt.Fprintf(&b, "cursor_hook_hit_rate %.6g\n", hitRate)
	return b.String()
}

func isGitMutation(event metrics.Event) bool {
	if event.Category == "git" && event.Action == "mutation" {
		return true
	}
	return event.Hook == "git-mutation"
}
