// Package mem0slo provides SLO tracking for Mem0 latency measurements.
package mem0slo

import (
    "sort"
    "sync"
    "time"
)

// SLOName identifies which SLO is being measured
type SLOName string

const (
    // SLOSearch tracks search operation latency
    SLOSearch SLOName = "search"

    // SLOAdd tracks add operation latency
    SLOAdd SLOName = "add"
)

// Sample represents a single latency measurement
type Sample struct {
    Name        SLOName
    DurationMS  int64
    RecordedAt  time.Time
}

// SLOReport summarizes SLO compliance for a specific operation
type SLOReport struct {
    Name        SLOName
    P95MS       int64
    TargetMS    int64
    Breached    bool
    SampleCount int
}

// Tracker collects and manages latency samples
type Tracker struct {
    mu      sync.Mutex
    samples []Sample
}

// NewTracker creates a new SLO tracker
func NewTracker() *Tracker {
    return &Tracker{
        samples: []Sample{},
    }
}

// Record adds a new sample to the tracker
func (t *Tracker) Record(name SLOName, durationMS int64) {
    t.mu.Lock()
    defer t.mu.Unlock()

    t.samples = append(t.samples, Sample{
        Name:       name,
        DurationMS: durationMS,
        RecordedAt: time.Now(),
    })
}

// Report computes the SLO report for a specific SLO name
func (t *Tracker) Report(name SLOName) SLOReport {
    t.mu.Lock()
    defer t.mu.Unlock()
    return t.reportLocked(name)
}

func (t *Tracker) reportLocked(name SLOName) SLOReport {
    var durations []int64
    for _, s := range t.samples {
        if s.Name == name {
            durations = append(durations, s.DurationMS)
        }
    }

    if len(durations) == 0 {
        return SLOReport{Name: name, TargetMS: name.defaultTarget()}
    }

    sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })

    p95Index := int(float64(len(durations)) * 0.95)
    if p95Index >= len(durations) {
        p95Index = len(durations) - 1
    }
    p95 := durations[p95Index]
    targetMS := name.defaultTarget()

    return SLOReport{
        Name:        name,
        P95MS:       p95,
        TargetMS:    targetMS,
        SampleCount: len(durations),
        Breached:    p95 > targetMS,
    }
}

// Alerts returns a list of all breached SLOs
func (t *Tracker) Alerts() []SLOReport {
    t.mu.Lock()
    defer t.mu.Unlock()

    var alerts []SLOReport
    for _, name := range []SLOName{SLOSearch, SLOAdd} {
        if r := t.reportLocked(name); r.Breached {
            alerts = append(alerts, r)
        }
    }
    return alerts
}

// defaultTarget returns the default target latency for a given SLO
func (name SLOName) defaultTarget() int64 {
    switch name {
    case SLOSearch:
        return 500 // ms
    case SLOAdd:
        return 200 // ms
    default:
        return 1000 // Default fallback
    }
}