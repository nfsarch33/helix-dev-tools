package mem0outbox

import (
	"sync"
	"time"
)

type OutboxStatus struct {
	Pending   int
	Delivered int
	Failed    int
}

type Entry struct {
	ID        string
	UserID    string
	AppID     string
	Content   string
	Metadata  map[string]string
	CreatedAt time.Time
	Delivered bool
	Attempts  int
}

type Outbox struct {
	mu      sync.Mutex
	entries []Entry
	seen    map[string]bool
}

func NewOutbox() *Outbox {
	return &Outbox{
		entries: make([]Entry, 0),
		seen:    make(map[string]bool),
	}
}

func (o *Outbox) Enqueue(e Entry) bool {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.seen[e.ID] {
		return false
	}

	o.entries = append(o.entries, e)
	o.seen[e.ID] = true
	return true
}

func (o *Outbox) Drain(deliverFn func(Entry) error) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	delivered := 0
	newEntries := make([]Entry, 0)

	for _, entry := range o.entries {
		if entry.Delivered {
			newEntries = append(newEntries, entry)
			continue
		}

		err := deliverFn(entry)
		if err != nil {
			entry.Attempts++
			newEntries = append(newEntries, entry)
		} else {
			entry.Delivered = true
			entry.Attempts++
			delivered++
			newEntries = append(newEntries, entry)
		}
	}

	o.entries = newEntries
	return delivered, nil
}

func (o *Outbox) Status() OutboxStatus {
	o.mu.Lock()
	defer o.mu.Unlock()

	status := OutboxStatus{}
	for _, entry := range o.entries {
		if entry.Delivered {
			status.Delivered++
		} else {
			status.Pending++
		}

		if entry.Attempts > 0 && !entry.Delivered {
			status.Failed++
		}
	}

	return status
}

func (o *Outbox) Alert() bool {
	o.mu.Lock()
	defer o.mu.Unlock()

	return len(o.entries) > 100
}