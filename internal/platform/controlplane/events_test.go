package controlplane

import (
	"sync/atomic"
	"testing"
)

func TestEventBus_PublishSubscribe(t *testing.T) {
	bus := NewEventBus(10)
	var received int32

	bus.Subscribe("test.event", func(e Event) {
		atomic.AddInt32(&received, 1)
	})

	bus.Publish("test.event", "payload")

	if atomic.LoadInt32(&received) != 1 {
		t.Error("expected handler to be called once")
	}
}

func TestEventBus_MultipleSubscribers(t *testing.T) {
	bus := NewEventBus(10)
	var count int32

	bus.Subscribe("evt", func(e Event) { atomic.AddInt32(&count, 1) })
	bus.Subscribe("evt", func(e Event) { atomic.AddInt32(&count, 1) })

	bus.Publish("evt", nil)

	if atomic.LoadInt32(&count) != 2 {
		t.Errorf("expected 2 handler calls, got %d", count)
	}
}

func TestEventBus_History(t *testing.T) {
	bus := NewEventBus(10)
	bus.Publish("a", "p1")
	bus.Publish("b", "p2")
	bus.Publish("a", "p3")

	all := bus.History("")
	if len(all) != 3 {
		t.Errorf("expected 3 events in history, got %d", len(all))
	}

	filtered := bus.History("a")
	if len(filtered) != 2 {
		t.Errorf("expected 2 'a' events, got %d", len(filtered))
	}
}

func TestEventBus_HistoryTruncation(t *testing.T) {
	bus := NewEventBus(3)
	for i := 0; i < 5; i++ {
		bus.Publish("evt", i)
	}

	history := bus.History("")
	if len(history) != 3 {
		t.Errorf("expected truncated to 3, got %d", len(history))
	}
}

func TestEventBus_SubscriberCount(t *testing.T) {
	bus := NewEventBus(10)
	bus.Subscribe("x", func(e Event) {})
	bus.Subscribe("x", func(e Event) {})
	bus.Subscribe("y", func(e Event) {})

	if bus.SubscriberCount("x") != 2 {
		t.Errorf("expected 2 subscribers for x")
	}
	if bus.SubscriberCount("z") != 0 {
		t.Errorf("expected 0 subscribers for z")
	}
}

func TestEventBus_NoSubscribers(t *testing.T) {
	bus := NewEventBus(10)
	bus.Publish("unhandled", "data")

	if len(bus.History("unhandled")) != 1 {
		t.Error("event should still be in history even without subscribers")
	}
}
