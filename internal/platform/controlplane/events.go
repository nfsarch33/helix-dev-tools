package controlplane

import (
	"sync"
	"time"
)

type Event struct {
	Type      string
	Payload   interface{}
	Timestamp time.Time
}

type Handler func(Event)

type EventBus struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
	history  []Event
	maxHist  int
}

func NewEventBus(maxHistory int) *EventBus {
	if maxHistory <= 0 {
		maxHistory = 100
	}
	return &EventBus{
		handlers: make(map[string][]Handler),
		maxHist:  maxHistory,
	}
}

func (b *EventBus) Subscribe(eventType string, handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventType] = append(b.handlers[eventType], handler)
}

func (b *EventBus) Publish(eventType string, payload interface{}) {
	event := Event{Type: eventType, Payload: payload, Timestamp: time.Now()}

	b.mu.Lock()
	b.history = append(b.history, event)
	if len(b.history) > b.maxHist {
		b.history = b.history[len(b.history)-b.maxHist:]
	}
	handlers := make([]Handler, len(b.handlers[eventType]))
	copy(handlers, b.handlers[eventType])
	b.mu.Unlock()

	for _, h := range handlers {
		h(event)
	}
}

func (b *EventBus) History(eventType string) []Event {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if eventType == "" {
		result := make([]Event, len(b.history))
		copy(result, b.history)
		return result
	}
	var filtered []Event
	for _, e := range b.history {
		if e.Type == eventType {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

func (b *EventBus) SubscriberCount(eventType string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.handlers[eventType])
}
