package hookmetrics

import (
	"reflect"
	"testing"
	"time"

	"github.com/nfsarch33/helix-dev-tools/internal/metrics"
)

func TestAudit_FleetHitRateAtLeast90Pct(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	events := []metrics.Event{
		{Timestamp: now, Category: "git", Action: "mutation", TurnID: "a", Profile: "runx"},
		{Timestamp: now, Hook: "post-shell", Action: "allow", TurnID: "a", Profile: "runx"},
		{Timestamp: now, Category: "git", Action: "mutation", TurnID: "b", Profile: "global-kb"},
		{Timestamp: now, Hook: "pre-push", Action: "allow", TurnID: "b", Profile: "global-kb"},
		{Timestamp: now, Category: "git", Action: "mutation", TurnID: "c", Profile: "agentic-research"},
	}

	result := Audit(events, now.Add(-time.Hour), 0.90)

	if result.Mutations != 3 || result.HookFires != 2 {
		t.Fatalf("counts = mutations %d fires %d", result.Mutations, result.HookFires)
	}
	if !result.BelowThreshold {
		t.Fatal("Audit must flag hit rate below 90%")
	}
	if !reflect.DeepEqual(result.Underperforming, []string{"agentic-research"}) {
		t.Fatalf("Underperforming = %#v", result.Underperforming)
	}
}
