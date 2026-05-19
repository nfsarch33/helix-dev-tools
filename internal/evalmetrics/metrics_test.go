package evalmetrics

import (
	"testing"
	"time"
)

func TestCollector(t *testing.T) {
	c := NewCollector("v6310", "cursor-parent")
	if c.SprintID() != "v6310" {
		t.Errorf("got sprint %q, want v6310", c.SprintID())
	}
}

func TestRecordPackageMetrics(t *testing.T) {
	c := NewCollector("v6310", "test")
	c.RecordPackage("evalharness", PackageMetrics{
		Tests:    6,
		Duration: 2 * time.Second,
		Coverage: 85.0,
		LOC:      120,
	})
	pkgs := c.Packages()
	if len(pkgs) != 1 {
		t.Fatalf("got %d packages, want 1", len(pkgs))
	}
	if pkgs["evalharness"].Tests != 6 {
		t.Errorf("got %d tests, want 6", pkgs["evalharness"].Tests)
	}
}

func TestAggregateMetrics(t *testing.T) {
	c := NewCollector("v6310", "test")
	c.RecordPackage("pkg1", PackageMetrics{Tests: 6, Duration: 2 * time.Second, LOC: 100})
	c.RecordPackage("pkg2", PackageMetrics{Tests: 8, Duration: 3 * time.Second, LOC: 150})

	agg := c.Aggregate()
	if agg.TotalTests != 14 {
		t.Errorf("got %d total tests, want 14", agg.TotalTests)
	}
	if agg.TotalLOC != 250 {
		t.Errorf("got %d total LOC, want 250", agg.TotalLOC)
	}
	if agg.TotalPackages != 2 {
		t.Errorf("got %d packages, want 2", agg.TotalPackages)
	}
}

func TestVelocity(t *testing.T) {
	c := NewCollector("v6310", "test")
	c.RecordPackage("pkg1", PackageMetrics{Tests: 6, Duration: 5 * time.Minute, LOC: 100})
	c.RecordPackage("pkg2", PackageMetrics{Tests: 8, Duration: 5 * time.Minute, LOC: 150})

	vel := c.Velocity()
	if vel.PackagesPerHour < 11.9 || vel.PackagesPerHour > 12.1 {
		t.Errorf("got velocity %f pkg/h, want ~12.0", vel.PackagesPerHour)
	}
}
