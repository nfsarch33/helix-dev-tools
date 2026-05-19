package observe

import (
	"testing"
	"time"
)

func TestTokenBudget_RecordSuccess(t *testing.T) {
	b := NewTokenBudget(1000, 1.0, BudgetPeriod{Start: time.Now(), End: time.Now().Add(24 * time.Hour)})
	ok := b.Record(100, 0.01)
	if !ok {
		t.Error("expected record to succeed")
	}
}

func TestTokenBudget_TokenExhaustion(t *testing.T) {
	b := NewTokenBudget(100, 10.0, BudgetPeriod{})
	b.Record(90, 0)
	ok := b.Record(20, 0)
	if ok {
		t.Error("expected record to fail when exceeding token limit")
	}
}

func TestTokenBudget_CostExhaustion(t *testing.T) {
	b := NewTokenBudget(100000, 0.50, BudgetPeriod{})
	b.Record(100, 0.45)
	ok := b.Record(100, 0.10)
	if ok {
		t.Error("expected record to fail when exceeding cost limit")
	}
}

func TestTokenBudget_RemainingTokens(t *testing.T) {
	b := NewTokenBudget(1000, 1.0, BudgetPeriod{})
	b.Record(300, 0)
	if b.RemainingTokens() != 700 {
		t.Errorf("expected 700, got %d", b.RemainingTokens())
	}
}

func TestTokenBudget_IsExhausted(t *testing.T) {
	b := NewTokenBudget(100, 1.0, BudgetPeriod{})
	b.Record(100, 0)
	if !b.IsExhausted() {
		t.Error("expected exhausted")
	}
}

func TestTokenBudget_UsagePercent(t *testing.T) {
	b := NewTokenBudget(200, 1.0, BudgetPeriod{})
	b.Record(100, 0)
	pct := b.UsagePercent()
	if pct < 49 || pct > 51 {
		t.Errorf("expected ~50%%, got %.1f%%", pct)
	}
}

func TestTokenBudget_Reset(t *testing.T) {
	b := NewTokenBudget(100, 1.0, BudgetPeriod{})
	b.Record(50, 0.5)
	b.Reset()
	if b.RemainingTokens() != 100 {
		t.Error("expected full budget after reset")
	}
}
