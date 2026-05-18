package mem0slo

import (
    "testing"
)

func TestRecord_StoresSamples(t *testing.T) {
    tracker := NewTracker()
    for i := 0; i < 5; i++ {
        tracker.Record(SLOSearch, 100)
    }

    report := tracker.Report(SLOSearch)
    if report.SampleCount != 5 {
        t.Errorf("Expected 5 samples, got %d", report.SampleCount)
    }
}

func TestReport_P95_Search(t *testing.T) {
    tracker := NewTracker()

    // 1 fast sample + 19 slow samples: P95 index = int(0.95*20) = 19 => 600ms
    tracker.Record(SLOSearch, 100)
    for i := 0; i < 19; i++ {
        tracker.Record(SLOSearch, 600)
    }

    report := tracker.Report(SLOSearch)
    if report.P95MS != 600 {
        t.Errorf("Expected P95 to be 600ms, got %d", report.P95MS)
    }

    if !report.Breached {
        t.Errorf("Expected SLO to be breached, but it was not")
    }
}

func TestReport_BelowTarget(t *testing.T) {
    tracker := NewTracker()

    // 10 samples below target
    for i := 0; i < 10; i++ {
        tracker.Record(SLOSearch, 100)
    }

    report := tracker.Report(SLOSearch)
    if report.Breached {
        t.Errorf("Expected no SLO breach, but it was breached")
    }
}

func TestReport_Breached(t *testing.T) {
    tracker := NewTracker()

    // Samples above 500ms
    for i := 0; i < 10; i++ {
        tracker.Record(SLOSearch, 600)
    }

    report := tracker.Report(SLOSearch)
    if !report.Breached {
        t.Errorf("Expected SLO to be breached, but it was not")
    }
}

func TestAlerts_ReturnsBreached(t *testing.T) {
    tracker := NewTracker()

    // Breach search SLO
    for i := 0; i < 10; i++ {
        tracker.Record(SLOSearch, 600)
    }

    // Keep add SLO clean
    for i := 0; i < 10; i++ {
        tracker.Record(SLOAdd, 100)
    }

    alerts := tracker.Alerts()
    if len(alerts) != 1 {
        t.Errorf("Expected 1 breached SLO, got %d", len(alerts))
    }

    if alerts[0].Name != SLOSearch {
        t.Errorf("Expected Search SLO in alerts, got %v", alerts[0].Name)
    }
}

func TestReport_EmptySamples(t *testing.T) {
    tracker := NewTracker()
    report := tracker.Report(SLOSearch)

    if report.SampleCount != 0 {
        t.Errorf("Expected 0 samples, got %d", report.SampleCount)
    }

    if report.Breached {
        t.Errorf("Expected no breach with no samples, but it was breached")
    }
}