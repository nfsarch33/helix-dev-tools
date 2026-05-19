package observe

import (
	"fmt"
	"sync"
	"time"
)

type BudgetPeriod struct {
	Start time.Time
	End   time.Time
}

type TokenBudget struct {
	mu        sync.RWMutex
	limit     int64
	used      int64
	costLimit float64
	costUsed  float64
	period    BudgetPeriod
}

func NewTokenBudget(tokenLimit int64, costLimit float64, period BudgetPeriod) *TokenBudget {
	return &TokenBudget{
		limit:     tokenLimit,
		costLimit: costLimit,
		period:    period,
	}
}

func (b *TokenBudget) Record(tokens int64, cost float64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.used+tokens > b.limit {
		return false
	}
	if b.costLimit > 0 && b.costUsed+cost > b.costLimit {
		return false
	}
	b.used += tokens
	b.costUsed += cost
	return true
}

func (b *TokenBudget) RemainingTokens() int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.limit - b.used
}

func (b *TokenBudget) RemainingCost() float64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.costLimit - b.costUsed
}

func (b *TokenBudget) UsagePercent() float64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.limit == 0 {
		return 0
	}
	return float64(b.used) / float64(b.limit) * 100
}

func (b *TokenBudget) IsExhausted() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.used >= b.limit || (b.costLimit > 0 && b.costUsed >= b.costLimit)
}

func (b *TokenBudget) Summary() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return fmt.Sprintf("tokens: %d/%d (%.1f%%), cost: $%.4f/$%.4f",
		b.used, b.limit, float64(b.used)/float64(b.limit)*100,
		b.costUsed, b.costLimit)
}

func (b *TokenBudget) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.used = 0
	b.costUsed = 0
}
