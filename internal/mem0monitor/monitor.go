package mem0monitor

import (
	"sync"
	"time"
)

type AlertLevel int

const (
	AlertWarn AlertLevel = iota
	AlertCrit
)

type Config struct {
	Endpoint      string
	Interval      time.Duration
	LatencyWarnMs int
	LatencyCritMs int
}

type Probe struct {
	Timestamp time.Time
	Latency   time.Duration
	Healthy   bool
	Operation string
	Error     string
}

type Alert struct {
	Level     AlertLevel
	Message   string
	Probe     Probe
	Timestamp time.Time
}

type Monitor struct {
	config Config
	mu     sync.Mutex
	probes []Probe
}

func New(cfg Config) *Monitor {
	if cfg.LatencyWarnMs == 0 {
		cfg.LatencyWarnMs = 2000
	}
	if cfg.LatencyCritMs == 0 {
		cfg.LatencyCritMs = 30000
	}
	return &Monitor{config: cfg}
}

func (m *Monitor) RecordProbe(p Probe) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p.Timestamp.IsZero() {
		p.Timestamp = time.Now()
	}
	m.probes = append(m.probes, p)
}

func (m *Monitor) Probes() []Probe {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Probe, len(m.probes))
	copy(out, m.probes)
	return out
}

func (m *Monitor) CheckAlerts() []Alert {
	m.mu.Lock()
	defer m.mu.Unlock()
	var alerts []Alert
	for _, p := range m.probes {
		ms := p.Latency.Milliseconds()
		if ms >= int64(m.config.LatencyCritMs) || !p.Healthy {
			alerts = append(alerts, Alert{
				Level:     AlertCrit,
				Message:   "critical latency or unhealthy",
				Probe:     p,
				Timestamp: time.Now(),
			})
		} else if ms >= int64(m.config.LatencyWarnMs) {
			alerts = append(alerts, Alert{
				Level:     AlertWarn,
				Message:   "elevated latency",
				Probe:     p,
				Timestamp: time.Now(),
			})
		}
	}
	return alerts
}

func (m *Monitor) AvgLatency() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.probes) == 0 {
		return 0
	}
	var total time.Duration
	for _, p := range m.probes {
		total += p.Latency
	}
	return total / time.Duration(len(m.probes))
}

func (m *Monitor) HealthRate() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.probes) == 0 {
		return 0
	}
	healthy := 0
	for _, p := range m.probes {
		if p.Healthy {
			healthy++
		}
	}
	return float64(healthy) / float64(len(m.probes))
}
