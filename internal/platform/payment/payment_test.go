package payment

import (
	"testing"
)

func TestMockProcessor_Success(t *testing.T) {
	p := &MockProcessor{ShouldFail: false}
	req := PaymentRequest{
		OrderID:   "test-order-1",
		Amount:    1000,
		Currency: "USD",
		Provider: ProviderMock,
	}

	result := p.Process(req)
	if result.Status != StatusSuccess {
		t.Errorf("Expected success status, got %v", result.Status)
	}
	if result.TransactionID != "mock-test-order-1" {
		t.Errorf("Incorrect transaction ID, got %v", result.TransactionID)
	}
}

func TestMockProcessor_Failure(t *testing.T) {
	p := &MockProcessor{ShouldFail: true}
	req := PaymentRequest{
		OrderID:   "test-order-2",
		Amount:    1000,
		Currency: "USD",
		Provider: ProviderMock,
	}

	result := p.Process(req)
	if result.Status != StatusFailed {
		t.Errorf("Expected failure status, got %v", result.Status)
	}
	if result.Error != "mock failure" {
		t.Errorf("Incorrect error message, got %v", result.Error)
	}
}

func TestEventLog_Record(t *testing.T) {
	log := NewEventLog()
	result1 := PaymentResult{
		TransactionID: "txn-1",
		Status:        StatusSuccess,
	}
	result2 := PaymentResult{
		TransactionID: "txn-2",
		Status:        StatusFailed,
	}

	log.Record(result1)
	log.Record(result2)

	events := log.All()
	if len(events) != 2 {
		t.Errorf("Expected 2 events, got %d", len(events))
	}
}

func TestEventLog_ByStatus_FiltersCorrectly(t *testing.T) {
	log := NewEventLog()
	log.Record(PaymentResult{
		TransactionID: "txn-1",
		Status:        StatusSuccess,
	})
	log.Record(PaymentResult{
		TransactionID: "txn-2",
		Status:        StatusFailed,
	})
	log.Record(PaymentResult{
		TransactionID: "txn-3",
		Status:        StatusSuccess,
	})

	successEvents := log.ByStatus(StatusSuccess)
	if len(successEvents) != 2 {
		t.Errorf("Expected 2 successful events, got %d", len(successEvents))
	}

	failedEvents := log.ByStatus(StatusFailed)
	if len(failedEvents) != 1 {
		t.Errorf("Expected 1 failed event, got %d", len(failedEvents))
	}
}

func TestEventLog_Empty(t *testing.T) {
	log := NewEventLog()
	events := log.All()
	if len(events) != 0 {
		t.Errorf("Expected empty log, got %d events", len(events))
	}
}