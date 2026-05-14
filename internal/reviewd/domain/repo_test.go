package domain

import (
	"encoding/json"
	"testing"
)

func TestTierString(t *testing.T) {
	tests := []struct {
		tier Tier
		want string
	}{
		{TierExcluded, "excluded"},
		{TierPilot, "pilot"},
		{TierA, "a"},
		{TierB, "b"},
		{TierC, "c"},
	}
	for _, tt := range tests {
		if got := tt.tier.String(); got != tt.want {
			t.Errorf("Tier(%d).String() = %q, want %q", tt.tier, got, tt.want)
		}
	}
}

func TestParseTier(t *testing.T) {
	tests := []struct {
		input string
		want  Tier
		ok    bool
	}{
		{"excluded", TierExcluded, true},
		{"PILOT", TierPilot, true},
		{" A ", TierA, true},
		{"b", TierB, true},
		{"c", TierC, true},
		{"unknown", TierExcluded, false},
		{"", TierExcluded, false},
	}
	for _, tt := range tests {
		got, ok := ParseTier(tt.input)
		if got != tt.want || ok != tt.ok {
			t.Errorf("ParseTier(%q) = (%v, %v), want (%v, %v)",
				tt.input, got, ok, tt.want, tt.ok)
		}
	}
}

func TestTierAllowsIssueWrite(t *testing.T) {
	if TierExcluded.AllowsIssueWrite() {
		t.Error("excluded should not allow issue writes")
	}
	if TierPilot.AllowsIssueWrite() {
		t.Error("pilot should not allow issue writes")
	}
	if !TierA.AllowsIssueWrite() {
		t.Error("tier A should allow issue writes")
	}
	if !TierB.AllowsIssueWrite() {
		t.Error("tier B should allow issue writes")
	}
	if !TierC.AllowsIssueWrite() {
		t.Error("tier C should allow issue writes")
	}
}

func TestTierJSON(t *testing.T) {
	r := Repo{Alias: "test", Tier: TierA, DefaultRef: "main"}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var r2 Repo
	if err := json.Unmarshal(b, &r2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if r2.Tier != TierA {
		t.Errorf("round-trip tier = %v, want %v", r2.Tier, TierA)
	}
}

func TestTierUnmarshalBadValue(t *testing.T) {
	var tier Tier
	err := json.Unmarshal([]byte(`"bogus"`), &tier)
	if err == nil {
		t.Error("expected error for unknown tier")
	}
}
