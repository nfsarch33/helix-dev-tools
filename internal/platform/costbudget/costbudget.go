package costbudget

import (
	"fmt"
	"sync"
	"time"
)

type Period string

const (
	PeriodDaily   Period = "daily"
	PeriodWeekly  Period = "weekly"
	PeriodMonthly Period = "monthly"
)

type Alert struct {
	Threshold float64 `json:"threshold"`
	Current   float64 `json:"current_pct"`
	Message   string  `json:"message"`
}

type Budget struct {
	Limit     float64   `json:"limit_usd"`
	Period    Period    `json:"period"`
	Spent     float64   `json:"spent_usd"`
	StartedAt time.Time `json:"started_at"`

	mu         sync.Mutex
	thresholds []float64
	byModel    map[string]float64
}

func NewBudget(limit float64, period Period) *Budget {
	return &Budget{
		Limit:     limit,
		Period:    period,
		StartedAt: time.Now(),
		byModel:   make(map[string]float64),
	}
}

func (b *Budget) SetAlertThresholds(thresholds ...float64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.thresholds = thresholds
}

func (b *Budget) RecordSpend(amount float64, model string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Spent += amount
	b.byModel[model] += amount
}

func (b *Budget) Remaining() float64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	r := b.Limit - b.Spent
	if r < 0 {
		return 0
	}
	return r
}

func (b *Budget) Exceeded() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Spent > b.Limit
}

func (b *Budget) CheckAlerts() []Alert {
	b.mu.Lock()
	defer b.mu.Unlock()

	pct := b.Spent / b.Limit
	var alerts []Alert
	for _, th := range b.thresholds {
		if pct >= th {
			alerts = append(alerts, Alert{
				Threshold: th,
				Current:   pct,
				Message:   fmt.Sprintf("%s budget %.0f%% used ($%.2f / $%.2f)", b.Period, pct*100, b.Spent, b.Limit),
			})
		}
	}
	return alerts
}

func (b *Budget) SpendByModel() map[string]float64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make(map[string]float64, len(b.byModel))
	for k, v := range b.byModel {
		out[k] = v
	}
	return out
}

func (b *Budget) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Spent = 0
	b.byModel = make(map[string]float64)
	b.StartedAt = time.Now()
}
