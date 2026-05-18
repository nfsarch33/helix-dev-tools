package promrules

import "testing"

func TestAdd_Get_Rule(t *testing.T) {
	rs := NewRuleSet("helixon-alerts")
	rs.Add(AlertRule{Name: "Mem0SearchSlow", Expr: `p95(mem0_search_ms) > 500`, Severity: SeverityWarning})
	r, ok := rs.Get("Mem0SearchSlow")
	if !ok {
		t.Fatal("expected to find rule")
	}
	if r.Severity != SeverityWarning {
		t.Errorf("expected warning, got %s", r.Severity)
	}
}

func TestGet_NotFound(t *testing.T) {
	rs := NewRuleSet("empty")
	_, ok := rs.Get("missing")
	if ok {
		t.Error("expected false for missing rule")
	}
}

func TestBySeverity(t *testing.T) {
	rs := NewRuleSet("test")
	rs.Add(AlertRule{Name: "A", Severity: SeverityCritical, Expr: "1"})
	rs.Add(AlertRule{Name: "B", Severity: SeverityWarning, Expr: "2"})
	rs.Add(AlertRule{Name: "C", Severity: SeverityCritical, Expr: "3"})
	crit := rs.BySeverity(SeverityCritical)
	if len(crit) != 2 {
		t.Errorf("expected 2 critical rules, got %d", len(crit))
	}
}

func TestValidate_Valid(t *testing.T) {
	rs := NewRuleSet("valid")
	rs.Add(AlertRule{Name: "A", Expr: "up == 0"})
	if errs := rs.Validate(); len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidate_MissingExpr(t *testing.T) {
	rs := NewRuleSet("invalid")
	rs.Add(AlertRule{Name: "NoExpr"})
	if errs := rs.Validate(); len(errs) == 0 {
		t.Error("expected error for missing expression")
	}
}

func TestAll_ReturnsCopy(t *testing.T) {
	rs := NewRuleSet("test")
	rs.Add(AlertRule{Name: "A", Expr: "1"})
	rs.Add(AlertRule{Name: "B", Expr: "2"})
	all := rs.All()
	if len(all) != 2 {
		t.Errorf("expected 2 rules, got %d", len(all))
	}
}
