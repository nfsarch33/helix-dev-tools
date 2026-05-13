package mem0cache

import (
	"log/slog"
	"sync"
	"time"
)

// AddEntry represents a single Mem0 add request.
type AddEntry struct {
	Content  string
	Metadata map[string]interface{}
	AddedAt  time.Time
}

// BatchStats reports batch buffer state.
type BatchStats struct {
	Pending int
	Flushed int64
	Errors  int64
}

// FlushFunc is called to send accumulated entries upstream.
type FlushFunc func(entries []AddEntry) error

// Batch accumulates add calls and flushes them periodically or when
// the buffer reaches a threshold.
type Batch struct {
	mu        sync.Mutex
	buf       []AddEntry
	threshold int
	interval  time.Duration
	flushFn   FlushFunc

	flushed int64
	errors  int64

	stopTicker chan struct{}
	done       chan struct{}
}

// BatchConfig controls batch behavior.
type BatchConfig struct {
	Threshold     int           // flush when buffer reaches this size (default 20)
	FlushInterval time.Duration // flush at this interval (default 60s)
	FlushFunc     FlushFunc     // called on flush; nil means entries are discarded
}

// NewBatch creates a write-behind buffer with a periodic flush goroutine.
func NewBatch(cfg BatchConfig) *Batch {
	if cfg.Threshold <= 0 {
		cfg.Threshold = 20
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 60 * time.Second
	}
	if cfg.FlushFunc == nil {
		cfg.FlushFunc = func([]AddEntry) error { return nil }
	}

	b := &Batch{
		buf:        make([]AddEntry, 0, cfg.Threshold),
		threshold:  cfg.Threshold,
		interval:   cfg.FlushInterval,
		flushFn:    cfg.FlushFunc,
		stopTicker: make(chan struct{}),
		done:       make(chan struct{}),
	}

	go b.tickerLoop()
	return b
}

// Add enqueues an entry. Triggers a flush if the buffer is full.
func (b *Batch) Add(entry AddEntry) {
	if entry.AddedAt.IsZero() {
		entry.AddedAt = time.Now()
	}

	b.mu.Lock()
	b.buf = append(b.buf, entry)
	shouldFlush := len(b.buf) >= b.threshold
	b.mu.Unlock()

	if shouldFlush {
		_ = b.Flush()
	}
}

// Flush sends all buffered entries to the upstream FlushFunc.
func (b *Batch) Flush() error {
	b.mu.Lock()
	if len(b.buf) == 0 {
		b.mu.Unlock()
		return nil
	}
	entries := b.buf
	b.buf = make([]AddEntry, 0, b.threshold)
	b.mu.Unlock()

	slog.Debug("batch flush", "count", len(entries))

	if err := b.flushFn(entries); err != nil {
		b.mu.Lock()
		b.errors++
		b.mu.Unlock()
		slog.Warn("batch flush error", "err", err, "count", len(entries))
		return err
	}

	b.mu.Lock()
	b.flushed += int64(len(entries))
	b.mu.Unlock()
	return nil
}

// Shutdown stops the ticker and flushes remaining entries.
func (b *Batch) Shutdown() {
	close(b.stopTicker)
	<-b.done
	_ = b.Flush()
}

// Stats returns current batch statistics.
func (b *Batch) Stats() BatchStats {
	b.mu.Lock()
	defer b.mu.Unlock()
	return BatchStats{
		Pending: len(b.buf),
		Flushed: b.flushed,
		Errors:  b.errors,
	}
}

func (b *Batch) tickerLoop() {
	defer close(b.done)
	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()

	for {
		select {
		case <-b.stopTicker:
			return
		case <-ticker.C:
			_ = b.Flush()
		}
	}
}
