package domain

import (
	"encoding/json"
	"testing"
)

func TestSeverityString(t *testing.T) {
	tests := []struct {
		sev  Severity
		want string
	}{
		{SeverityNit, "nit"},
		{SeverityShould, "should"},
		{SeverityMust, "must"},
	}
	for _, tt := range tests {
		if got := tt.sev.String(); got != tt.want {
			t.Errorf("Severity(%d).String() = %q, want %q", tt.sev, got, tt.want)
		}
	}
}

func TestParseSeverity(t *testing.T) {
	tests := []struct {
		input string
		want  Severity
		ok    bool
	}{
		{"nit", SeverityNit, true},
		{"info", SeverityNit, true},
		{"should", SeverityShould, true},
		{"WARNING", SeverityShould, true},
		{"must", SeverityMust, true},
		{"CRITICAL", SeverityMust, true},
		{"unknown", SeverityNit, false},
		{"", SeverityNit, false},
	}
	for _, tt := range tests {
		got, ok := ParseSeverity(tt.input)
		if got != tt.want || ok != tt.ok {
			t.Errorf("ParseSeverity(%q) = (%v, %v), want (%v, %v)",
				tt.input, got, ok, tt.want, tt.ok)
		}
	}
}

func TestSeverityJSON(t *testing.T) {
	f := Finding{File: "main.go", Severity: SeverityMust, Message: "test"}
	b, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var f2 Finding
	if err := json.Unmarshal(b, &f2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if f2.Severity != SeverityMust {
		t.Errorf("round-trip severity = %v, want %v", f2.Severity, SeverityMust)
	}
}

func TestSeverityUnmarshalBadValue(t *testing.T) {
	var sev Severity
	err := json.Unmarshal([]byte(`"bogus"`), &sev)
	if err == nil {
		t.Error("expected error for unknown severity")
	}
}

func TestCountFindings(t *testing.T) {
	findings := []Finding{
		{File: "a.go", Severity: SeverityMust, Message: "1"},
		{File: "b.go", Severity: SeverityMust, Message: "2"},
		{File: "c.go", Severity: SeverityShould, Message: "3"},
		{File: "d.go", Severity: SeverityNit, Message: "4"},
		{File: "e.go", Severity: SeverityNit, Message: "5"},
		{File: "f.go", Severity: SeverityNit, Message: "6"},
	}
	c := CountFindings(findings)
	if c.Must != 2 || c.Should != 1 || c.Nit != 3 {
		t.Errorf("counts = %+v, want {Must:2 Should:1 Nit:3}", c)
	}
	if c.Total() != 6 {
		t.Errorf("Total() = %d, want 6", c.Total())
	}
}

func TestCountFindingsEmpty(t *testing.T) {
	c := CountFindings(nil)
	if c.Total() != 0 {
		t.Errorf("empty Total() = %d, want 0", c.Total())
	}
}
