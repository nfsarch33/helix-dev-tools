package domain

import (
	"testing"
	"time"
)

func TestCycleHasNewCommits(t *testing.T) {
	tests := []struct {
		name string
		head string
		base string
		want bool
	}{
		{"different SHAs", "abc123", "def456", true},
		{"same SHA", "abc123", "abc123", false},
		{"empty head", "", "def456", false},
		{"both empty", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Cycle{HeadSHA: tt.head, BaseSHA: tt.base}
			if got := c.HasNewCommits(); got != tt.want {
				t.Errorf("HasNewCommits() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCycleNeedsEscalation(t *testing.T) {
	tests := []struct {
		name string
		loc  int
		want bool
	}{
		{"below threshold", 100, false},
		{"at threshold", 300, true},
		{"above threshold", 500, true},
		{"zero", 0, false},
		{"negative", -50, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Cycle{LOCDelta: tt.loc}
			if got := c.NeedsEscalation(); got != tt.want {
				t.Errorf("NeedsEscalation() = %v, want %v (loc=%d)",
					got, tt.want, tt.loc)
			}
		})
	}
}

func TestCycleStateString(t *testing.T) {
	states := []struct {
		state CycleState
		want  string
	}{
		{CycleIdle, "idle"},
		{CycleScanning, "scanning"},
		{CycleReporting, "reporting"},
		{CycleDone, "done"},
		{CycleEscalated, "escalated"},
		{CycleSkipped, "skipped"},
	}
	for _, tt := range states {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("CycleState(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestCycleFieldsPopulated(t *testing.T) {
	now := time.Now()
	c := Cycle{
		RepoAlias: "cursor-tools",
		State:     CycleScanning,
		HeadSHA:   "abc123",
		BaseSHA:   "def456",
		LOCDelta:  42,
		Findings: []Finding{
			{File: "main.go", Severity: SeverityMust, Message: "test"},
		},
		StartedAt: now,
	}
	if c.RepoAlias != "cursor-tools" {
		t.Error("alias mismatch")
	}
	if !c.HasNewCommits() {
		t.Error("should have new commits")
	}
	if c.NeedsEscalation() {
		t.Error("42 LOC should not escalate")
	}
	if len(c.Findings) != 1 {
		t.Error("expected 1 finding")
	}
}
