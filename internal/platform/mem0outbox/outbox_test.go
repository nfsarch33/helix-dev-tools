package mem0outbox

import (
	"errors"
	"testing"
	"time"
)

func TestEnqueue_AddsEntry(t *testing.T) {
	t.Parallel()
	o := NewOutbox()
	e := Entry{
		ID:        "test-1",
		UserID:    "user-1",
		AppID:     "app-1",
		Content:   "test content",
		CreatedAt: time.Now(),
	}

	result := o.Enqueue(e)
	if !result {
		t.Errorf("Enqueue returned false, want true")
	}

	status := o.Status()
	if status.Pending != 1 {
		t.Errorf("Status.Pending = %d, want 1", status.Pending)
	}
}

func TestEnqueue_Dedup(t *testing.T) {
	t.Parallel()
	o := NewOutbox()
	e := Entry{
		ID:        "test-1",
		UserID:    "user-1",
		AppID:     "app-1",
		Content:   "test content",
		CreatedAt: time.Now(),
	}

	first := o.Enqueue(e)
	second := o.Enqueue(e)

	if !first {
		t.Errorf("First Enqueue returned false, want true")
	}

	if second {
		t.Errorf("Second Enqueue returned true, want false")
	}

	status := o.Status()
	if status.Pending != 1 {
		t.Errorf("Status.Pending = %d, want 1", status.Pending)
	}
}

func TestDrain_DeliversAll(t *testing.T) {
	t.Parallel()
	o := NewOutbox()

	entries := []Entry{
		{ID: "test-1", UserID: "user-1", AppID: "app-1"},
		{ID: "test-2", UserID: "user-2", AppID: "app-2"},
	}

	for _, e := range entries {
		o.Enqueue(e)
	}

	noopDeliverFn := func(entry Entry) error {
		return nil
	}

	delivered, err := o.Drain(noopDeliverFn)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if delivered != 2 {
		t.Errorf("Delivered = %d, want 2", delivered)
	}

	status := o.Status()
	if status.Pending != 0 {
		t.Errorf("Status.Pending = %d, want 0", status.Pending)
	}
	if status.Delivered != 2 {
		t.Errorf("Status.Delivered = %d, want 2", status.Delivered)
	}
}

func TestDrain_PartialFailure(t *testing.T) {
	t.Parallel()
	o := NewOutbox()

	entries := []Entry{
		{ID: "test-1", UserID: "user-1", AppID: "app-1"},
		{ID: "test-2", UserID: "user-2", AppID: "app-2"},
	}

	for _, e := range entries {
		o.Enqueue(e)
	}

	deliverFn := func(entry Entry) error {
		if entry.ID == "test-2" {
			return errors.New("delivery failed")
		}
		return nil
	}

	delivered, err := o.Drain(deliverFn)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if delivered != 1 {
		t.Errorf("Delivered = %d, want 1", delivered)
	}

	status := o.Status()
	if status.Delivered != 1 {
		t.Errorf("Status.Delivered = %d, want 1", status.Delivered)
	}
	if status.Failed != 1 {
		t.Errorf("Status.Failed = %d, want 1", status.Failed)
	}
	if status.Pending == 0 {
		t.Errorf("Status.Pending = %d, want > 0", status.Pending)
	}
}

func TestAlert_NotTriggered(t *testing.T) {
	t.Parallel()
	o := NewOutbox()

	for i := 0; i < 50; i++ {
		o.Enqueue(Entry{
			ID:        "test-" + string(rune(i)),
			UserID:    "user-1",
			AppID:     "app-1",
		})
	}

	if o.Alert() {
		t.Errorf("Alert() returned true for 50 entries, want false")
	}
}

func TestAlert_Triggered(t *testing.T) {
	t.Parallel()
	o := NewOutbox()

	for i := 0; i < 101; i++ {
		o.Enqueue(Entry{
			ID:        "test-" + string(rune(i)),
			UserID:    "user-1",
			AppID:     "app-1",
		})
	}

	if !o.Alert() {
		t.Errorf("Alert() returned false for 101 entries, want true")
	}
}

func TestStatus_Fields(t *testing.T) {
	t.Parallel()
	o := NewOutbox()

	// Some delivered, some failed
	entries := []struct {
		id        string
		delivered bool
		attempts  int
	}{
		{"test-1", true, 1},   // delivered
		{"test-2", false, 0},  // pending
		{"test-3", false, 1},  // failed
		{"test-4", false, 2},  // failed
	}

	for _, e := range entries {
		entry := Entry{
			ID:        e.id,
			UserID:    "user-1",
			AppID:     "app-1",
			Delivered: e.delivered,
			Attempts:  e.attempts,
		}
		o.Enqueue(entry)
	}

	status := o.Status()
	if status.Pending != 3 {
		t.Errorf("Status.Pending = %d, want 3", status.Pending)
	}
	if status.Delivered != 1 {
		t.Errorf("Status.Delivered = %d, want 1", status.Delivered)
	}
	if status.Failed != 2 {
		t.Errorf("Status.Failed = %d, want 2", status.Failed)
	}
}