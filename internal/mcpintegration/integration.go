package mcpintegration

import (
	"sync"
	"time"
)

type Config struct {
	Enabled     bool
	LogPath     string
	IncludeArgs bool
	Servers     []string
}

type ToolCall struct {
	Server    string
	Tool      string
	Duration  time.Duration
	Success   bool
	Error     string
	Timestamp time.Time
}

type ServerStat struct {
	TotalCalls   int
	FailureCount int
	AvgLatency   time.Duration
	TotalLatency time.Duration
}

type Tracer struct {
	config  Config
	mu      sync.Mutex
	calls   []ToolCall
	servers map[string]bool
}

func NewTracer(cfg Config) *Tracer {
	servers := make(map[string]bool)
	for _, s := range cfg.Servers {
		servers[s] = true
	}
	return &Tracer{config: cfg, servers: servers}
}

func (t *Tracer) RecordToolCall(tc ToolCall) {
	if !t.config.Enabled {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if tc.Timestamp.IsZero() {
		tc.Timestamp = time.Now()
	}
	t.calls = append(t.calls, tc)
}

func (t *Tracer) ToolCalls() []ToolCall {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]ToolCall, len(t.calls))
	copy(out, t.calls)
	return out
}

func (t *Tracer) ServerStats() map[string]*ServerStat {
	t.mu.Lock()
	defer t.mu.Unlock()
	stats := make(map[string]*ServerStat)
	for _, tc := range t.calls {
		s, ok := stats[tc.Server]
		if !ok {
			s = &ServerStat{}
			stats[tc.Server] = s
		}
		s.TotalCalls++
		s.TotalLatency += tc.Duration
		if !tc.Success {
			s.FailureCount++
		}
	}
	for _, s := range stats {
		if s.TotalCalls > 0 {
			s.AvgLatency = s.TotalLatency / time.Duration(s.TotalCalls)
		}
	}
	return stats
}

func (t *Tracer) IsServerTracked(server string) bool {
	if len(t.servers) == 0 {
		return true
	}
	return t.servers[server]
}
