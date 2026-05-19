package observe

import (
	"fmt"
	"sync"
	"time"
)

type MetricType int

const (
	MetricCounter MetricType = iota
	MetricGauge
	MetricHistogram
)

type Metric struct {
	Name      string
	Type      MetricType
	Value     float64
	Labels    map[string]string
	UpdatedAt time.Time
}

type MetricCollector struct {
	mu      sync.RWMutex
	metrics map[string]*Metric
}

func NewMetricCollector() *MetricCollector {
	return &MetricCollector{metrics: make(map[string]*Metric)}
}

func (c *MetricCollector) Counter(name string, labels map[string]string) {
	key := metricKey(name, labels)
	c.mu.Lock()
	defer c.mu.Unlock()
	m, ok := c.metrics[key]
	if !ok {
		c.metrics[key] = &Metric{Name: name, Type: MetricCounter, Value: 1, Labels: labels, UpdatedAt: time.Now()}
		return
	}
	m.Value++
	m.UpdatedAt = time.Now()
}

func (c *MetricCollector) Gauge(name string, value float64, labels map[string]string) {
	key := metricKey(name, labels)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics[key] = &Metric{Name: name, Type: MetricGauge, Value: value, Labels: labels, UpdatedAt: time.Now()}
}

func (c *MetricCollector) Get(name string) (float64, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, m := range c.metrics {
		if m.Name == name {
			return m.Value, true
		}
	}
	return 0, false
}

func (c *MetricCollector) All() []Metric {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]Metric, 0, len(c.metrics))
	for _, m := range c.metrics {
		result = append(result, *m)
	}
	return result
}

func (c *MetricCollector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics = make(map[string]*Metric)
}

func (c *MetricCollector) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.metrics)
}

func metricKey(name string, labels map[string]string) string {
	key := name
	for k, v := range labels {
		key += fmt.Sprintf("_%s_%s", k, v)
	}
	return key
}
