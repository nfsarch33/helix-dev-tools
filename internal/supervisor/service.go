package supervisor

import (
	"context"
	"time"
)

// Service is the contract every long-running daemon component implements.
// Run MUST block until ctx is cancelled and SHOULD return ctx.Err() on
// clean shutdown. Returning any other error signals a crash; the
// supervisor will log the failure and (subject to backoff) restart the
// service.
//
// Implementations MUST be safe to invoke Run multiple times across the
// lifetime of a supervisor: each crash triggers a fresh Run call after
// the backoff delay.
type Service interface {
	Name() string
	Run(ctx context.Context) error
}

// HealthState describes the latest known state of a registered service.
// The zero value is meaningful: Started=false means the service has
// never started yet.
type HealthState struct {
	Name        string
	Started     bool
	LastStarted time.Time
	LastExited  time.Time
	LastError   error
	Restarts    int
	Panicking   bool
}

// MemoryPressureProbe broadcasts the most recent system memory pressure
// reading to all subscribers. Single probe goroutine owned by the
// supervisor; Services that want pressure data Subscribe and read from
// the returned channel. The supervisor takes ownership of starting and
// stopping the probe.
type MemoryPressureProbe interface {
	Subscribe() <-chan MemoryPressure
	Run(ctx context.Context) error
}

// MemoryPressure is the snapshot delivered to each subscriber. Fields
// match the macOS memory_pressure(8) summary line.
type MemoryPressure struct {
	When    time.Time
	FreePct int
	Level   string // green | yellow | red
}
