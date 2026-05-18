package payment

import (
	"sync"
)

type Provider string

const (
	ProviderStripe Provider = "stripe"
	ProviderMock   Provider = "mock"
)

type PaymentStatus string

const (
	StatusPending PaymentStatus = "pending"
	StatusSuccess PaymentStatus = "success"
	StatusFailed  PaymentStatus = "failed"
)

type PaymentRequest struct {
	OrderID   string
	Amount    int64 // cents
	Currency string
	Provider Provider
}

type PaymentResult struct {
	TransactionID string
	Status        PaymentStatus
	Error         string
}

type Processor interface {
	Process(r PaymentRequest) PaymentResult
}

// MockProcessor is a test-only processor
type MockProcessor struct {
	ShouldFail bool
}

func (m *MockProcessor) Process(r PaymentRequest) PaymentResult {
	if m.ShouldFail {
		return PaymentResult{
			Status: StatusFailed,
			Error:  "mock failure",
		}
	}
	return PaymentResult{
		TransactionID: "mock-" + r.OrderID,
		Status:        StatusSuccess,
	}
}

// EventLog records payment events in memory
type EventLog struct {
	events []PaymentResult
	mu     sync.Mutex
}

func NewEventLog() *EventLog {
	return &EventLog{
		events: []PaymentResult{},
	}
}

func (e *EventLog) Record(r PaymentResult) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = append(e.events, r)
}

func (e *EventLog) All() []PaymentResult {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]PaymentResult{}, e.events...)
}

func (e *EventLog) ByStatus(s PaymentStatus) []PaymentResult {
	e.mu.Lock()
	defer e.mu.Unlock()

	var filtered []PaymentResult
	for _, event := range e.events {
		if event.Status == s {
			filtered = append(filtered, event)
		}
	}
	return filtered
}