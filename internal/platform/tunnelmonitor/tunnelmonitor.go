package tunnelmonitor

import (
	"fmt"
	"net"
	"sync"
	"time"
)

type ProbeResult struct {
	Timestamp time.Time     `json:"ts"`
	Healthy   bool          `json:"healthy"`
	Latency   time.Duration `json:"latency_ms"`
	Error     string        `json:"error,omitempty"`
}

type Monitor struct {
	Name               string        `json:"name"`
	Host               string        `json:"host"`
	Port               int           `json:"port"`
	Timeout            time.Duration `json:"timeout"`
	Healthy            bool          `json:"healthy"`
	ConsecutiveFails   int           `json:"consecutive_fails"`
	ReconnectThreshold int           `json:"reconnect_threshold"`
	LastProbe          time.Time     `json:"last_probe"`

	mu      sync.Mutex
	history []ProbeResult
}

func NewMonitor(name, host string, port int, timeout time.Duration) *Monitor {
	return &Monitor{
		Name:               name,
		Host:               host,
		Port:               port,
		Timeout:            timeout,
		ReconnectThreshold: 3,
	}
}

func (m *Monitor) Address() string {
	return fmt.Sprintf("%s:%d", m.Host, m.Port)
}

func (m *Monitor) Probe() ProbeResult {
	m.mu.Lock()
	defer m.mu.Unlock()

	start := time.Now()
	conn, err := net.DialTimeout("tcp", m.Address(), m.Timeout)
	latency := time.Since(start)

	result := ProbeResult{
		Timestamp: start,
		Latency:   latency,
	}

	if err != nil {
		result.Healthy = false
		result.Error = err.Error()
		m.Healthy = false
		m.ConsecutiveFails++
	} else {
		conn.Close()
		result.Healthy = true
		m.Healthy = true
		m.ConsecutiveFails = 0
	}

	m.LastProbe = start
	m.history = append(m.history, result)

	return result
}

func (m *Monitor) NeedsReconnect() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ConsecutiveFails >= m.ReconnectThreshold
}

func (m *Monitor) History() []ProbeResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]ProbeResult, len(m.history))
	copy(out, m.history)
	return out
}
