package autohandoff

import (
	"strings"
	"testing"
)

func TestOnSprintClose_AllDone(t *testing.T) {
	sprint := Sprint{
		Name: "v100",
		Tickets: []Ticket{
			{ID: "T-1", Title: "Feature A", Status: StatusDone},
			{ID: "T-2", Title: "Feature B", Status: StatusDone},
		},
	}

	md := OnSprintClose(sprint)

	if !strings.Contains(md, "# Handoff: v100") {
		t.Fatal("missing sprint name header")
	}
	if !strings.Contains(md, "Completed: 2") {
		t.Fatal("expected 2 completed")
	}
	if !strings.Contains(md, "Blocked: 0") {
		t.Fatal("expected 0 blocked")
	}
	if strings.Contains(md, "## Blocked") {
		t.Fatal("blocked section should not appear")
	}
}

func TestOnSprintClose_MixedStatus(t *testing.T) {
	sprint := Sprint{
		Name: "v101",
		Tickets: []Ticket{
			{ID: "T-1", Title: "Done item", Status: StatusDone},
			{ID: "T-2", Title: "Blocked item", Status: StatusBlocked},
			{ID: "T-3", Title: "Open item", Status: StatusOpen},
		},
	}

	md := OnSprintClose(sprint)

	if !strings.Contains(md, "Completed: 1") {
		t.Fatal("expected 1 completed")
	}
	if !strings.Contains(md, "Blocked: 1") {
		t.Fatal("expected 1 blocked")
	}
	if !strings.Contains(md, "## Blocked") {
		t.Fatal("missing blocked section")
	}
	if !strings.Contains(md, "## Carry-Forward") {
		t.Fatal("missing carry-forward section")
	}
}

func TestOnSprintClose_EmptySprint(t *testing.T) {
	sprint := Sprint{Name: "v102", Tickets: nil}

	md := OnSprintClose(sprint)

	if !strings.Contains(md, "Completed: 0") {
		t.Fatal("expected 0 completed")
	}
	if !strings.Contains(md, "# Handoff: v102") {
		t.Fatal("missing sprint name")
	}
}
