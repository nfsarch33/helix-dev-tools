package embedwarmup

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type ProbeResult struct {
	Timestamp time.Time     `json:"ts"`
	Healthy   bool          `json:"healthy"`
	Latency   time.Duration `json:"latency_ms"`
	Error     string        `json:"error,omitempty"`
}

type HealthReport struct {
	TotalProbes  int           `json:"total_probes"`
	SuccessCount int           `json:"success_count"`
	FailureCount int           `json:"failure_count"`
	AvgLatency   time.Duration `json:"avg_latency_ms"`
	LastProbe    *ProbeResult  `json:"last_probe,omitempty"`
}

type Prober struct {
	mu      sync.Mutex
	url     string
	timeout time.Duration
	client  *http.Client
	history []ProbeResult
}

func NewProber(url string, timeout time.Duration) *Prober {
	return &Prober{
		url:     url,
		timeout: timeout,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (p *Prober) Probe() ProbeResult {
	start := time.Now()
	result := ProbeResult{Timestamp: start}

	body := `{"input":"warmup","model":"pplx-embed-v1"}`
	resp, err := p.client.Post(p.url, "application/json", strings.NewReader(body))

	result.Latency = time.Since(start)

	if err != nil {
		result.Error = err.Error()
	} else {
		defer resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			result.Healthy = true
		} else {
			result.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
	}

	p.mu.Lock()
	p.history = append(p.history, result)
	p.mu.Unlock()

	return result
}

func (p *Prober) Report() HealthReport {
	p.mu.Lock()
	defer p.mu.Unlock()

	report := HealthReport{
		TotalProbes: len(p.history),
	}

	if len(p.history) == 0 {
		return report
	}

	var totalLatency time.Duration
	for i := range p.history {
		if p.history[i].Healthy {
			report.SuccessCount++
		} else {
			report.FailureCount++
		}
		totalLatency += p.history[i].Latency
	}
	report.AvgLatency = totalLatency / time.Duration(len(p.history))

	last := p.history[len(p.history)-1]
	report.LastProbe = &last

	return report
}
