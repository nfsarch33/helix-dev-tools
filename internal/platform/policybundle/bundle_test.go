package policybundle

import (
	"testing"
)

func TestRegisterAndGet(t *testing.T) {
	r := New()

	b := Bundle{
		ID:          "no-shell-leak",
		Name:        "No Shell Leak Policy",
		Description: "Prevents sensitive identifiers from appearing in shell argv",
		Severity:    SeverityBlock,
		Rules: []Rule{
			{ID: "no-ip-on-argv", Pattern: `\b100\.\d+\.\d+\.\d+\b`, Description: "Tailscale IP on argv"},
			{ID: "no-ssh-key-path", Pattern: `\.ssh/`, Description: "SSH key path on argv"},
		},
	}

	err := r.Register(b)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	got, err := r.Get("no-shell-leak")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "No Shell Leak Policy" {
		t.Errorf("got name %q", got.Name)
	}
	if len(got.Rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(got.Rules))
	}
}

func TestDuplicateBundle(t *testing.T) {
	r := New()
	r.Register(Bundle{ID: "x", Name: "X", Severity: SeverityWarn})
	err := r.Register(Bundle{ID: "x", Name: "X", Severity: SeverityWarn})
	if err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestGetNotFound(t *testing.T) {
	r := New()
	_, err := r.Get("missing")
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestListBySeverity(t *testing.T) {
	r := New()
	r.Register(Bundle{ID: "a", Name: "A", Severity: SeverityBlock})
	r.Register(Bundle{ID: "b", Name: "B", Severity: SeverityWarn})
	r.Register(Bundle{ID: "c", Name: "C", Severity: SeverityBlock})
	r.Register(Bundle{ID: "d", Name: "D", Severity: SeverityInfo})

	blocks := r.ListBySeverity(SeverityBlock)
	if len(blocks) != 2 {
		t.Errorf("expected 2 blocks, got %d", len(blocks))
	}

	warns := r.ListBySeverity(SeverityWarn)
	if len(warns) != 1 {
		t.Errorf("expected 1 warn, got %d", len(warns))
	}
}

func TestEvaluate(t *testing.T) {
	r := New()
	r.Register(Bundle{
		ID:       "test-policy",
		Name:     "Test",
		Severity: SeverityBlock,
		Rules: []Rule{
			{ID: "no-secret", Pattern: `SECRET_KEY`, Description: "Secret on argv"},
			{ID: "no-password", Pattern: `password=`, Description: "Password on argv"},
		},
	})

	violations := r.Evaluate("test-policy", "curl --header SECRET_KEY=abc http://example.com")
	if len(violations) != 1 {
		t.Errorf("expected 1 violation, got %d", len(violations))
	}
	if violations[0].RuleID != "no-secret" {
		t.Errorf("expected rule no-secret, got %q", violations[0].RuleID)
	}

	violations = r.Evaluate("test-policy", "echo hello world")
	if len(violations) != 0 {
		t.Errorf("expected 0 violations, got %d", len(violations))
	}
}

func TestEvaluateAll(t *testing.T) {
	r := New()
	r.Register(Bundle{
		ID:       "policy-a",
		Name:     "A",
		Severity: SeverityBlock,
		Rules:    []Rule{{ID: "r1", Pattern: `forbidden`, Description: "Forbidden word"}},
	})
	r.Register(Bundle{
		ID:       "policy-b",
		Name:     "B",
		Severity: SeverityWarn,
		Rules:    []Rule{{ID: "r2", Pattern: `caution`, Description: "Caution word"}},
	})

	results := r.EvaluateAll("this has forbidden and caution words")
	if len(results) != 2 {
		t.Errorf("expected 2 bundle results, got %d", len(results))
	}

	totalViolations := 0
	for _, br := range results {
		totalViolations += len(br.Violations)
	}
	if totalViolations != 2 {
		t.Errorf("expected 2 total violations, got %d", totalViolations)
	}
}

func TestValidateBundle(t *testing.T) {
	tests := []struct {
		name    string
		bundle  Bundle
		wantErr bool
	}{
		{"valid", Bundle{ID: "x", Name: "X", Severity: SeverityBlock}, false},
		{"empty id", Bundle{ID: "", Name: "X", Severity: SeverityBlock}, true},
		{"empty name", Bundle{ID: "x", Name: "", Severity: SeverityBlock}, true},
		{"invalid severity", Bundle{ID: "x", Name: "X", Severity: "critical"}, true},
		{"invalid rule pattern", Bundle{ID: "x", Name: "X", Severity: SeverityBlock, Rules: []Rule{{ID: "r", Pattern: "[invalid"}}}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.bundle)
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() err=%v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

func TestAll(t *testing.T) {
	r := New()
	r.Register(Bundle{ID: "a", Name: "A", Severity: SeverityBlock})
	r.Register(Bundle{ID: "b", Name: "B", Severity: SeverityWarn})

	all := r.All()
	if len(all) != 2 {
		t.Errorf("expected 2, got %d", len(all))
	}
}

func TestEnabled(t *testing.T) {
	r := New()
	r.Register(Bundle{ID: "on", Name: "On", Severity: SeverityBlock, Enabled: true})
	r.Register(Bundle{ID: "off", Name: "Off", Severity: SeverityBlock, Enabled: false})

	enabled := r.ListEnabled()
	if len(enabled) != 1 {
		t.Errorf("expected 1 enabled, got %d", len(enabled))
	}
	if enabled[0].ID != "on" {
		t.Errorf("expected 'on', got %q", enabled[0].ID)
	}
}
