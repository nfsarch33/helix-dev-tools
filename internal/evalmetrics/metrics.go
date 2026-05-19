package evalmetrics

import (
	"sync"
	"time"
)

type PackageMetrics struct {
	Tests    int
	Duration time.Duration
	Coverage float64
	LOC      int
}

type AggregateMetrics struct {
	TotalTests    int
	TotalLOC      int
	TotalPackages int
	TotalDuration time.Duration
	AvgCoverage   float64
}

type Velocity struct {
	PackagesPerHour float64
	TestsPerHour    float64
	LOCPerHour      float64
}

type Collector struct {
	sprintID string
	agentID  string
	mu       sync.Mutex
	packages map[string]PackageMetrics
}

func NewCollector(sprintID, agentID string) *Collector {
	return &Collector{
		sprintID: sprintID,
		agentID:  agentID,
		packages: make(map[string]PackageMetrics),
	}
}

func (c *Collector) SprintID() string { return c.sprintID }

func (c *Collector) RecordPackage(name string, m PackageMetrics) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.packages[name] = m
}

func (c *Collector) Packages() map[string]PackageMetrics {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]PackageMetrics, len(c.packages))
	for k, v := range c.packages {
		out[k] = v
	}
	return out
}

func (c *Collector) Aggregate() AggregateMetrics {
	c.mu.Lock()
	defer c.mu.Unlock()
	var agg AggregateMetrics
	var coverageSum float64
	for _, m := range c.packages {
		agg.TotalTests += m.Tests
		agg.TotalLOC += m.LOC
		agg.TotalDuration += m.Duration
		coverageSum += m.Coverage
		agg.TotalPackages++
	}
	if agg.TotalPackages > 0 {
		agg.AvgCoverage = coverageSum / float64(agg.TotalPackages)
	}
	return agg
}

func (c *Collector) Velocity() Velocity {
	agg := c.Aggregate()
	hours := agg.TotalDuration.Hours()
	if hours == 0 {
		return Velocity{}
	}
	return Velocity{
		PackagesPerHour: float64(agg.TotalPackages) / hours,
		TestsPerHour:    float64(agg.TotalTests) / hours,
		LOCPerHour:      float64(agg.TotalLOC) / hours,
	}
}
