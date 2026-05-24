package evalharness

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Daemon watches agentrace NDJSON files and grades events continuously.
type Daemon struct {
	cfg      DaemonConfig
	graders  []DeterministicGrader
	results  []GradeResult
	mu       sync.RWMutex
	cancel   context.CancelFunc
	done     chan struct{}
}

// DaemonConfig configures the eval daemon.
type DaemonConfig struct {
	AgentracePath string       `json:"agentrace_path"`
	PollInterval  time.Duration `json:"poll_interval"`
	GraderConfig  GraderConfig  `json:"grader_config"`
	GateConfig    GateConfig    `json:"gate_config"`
}

// DefaultDaemonConfig returns defaults suitable for background operation.
func DefaultDaemonConfig() DaemonConfig {
	return DaemonConfig{
		PollInterval: 5 * time.Second,
		GraderConfig: DefaultGraderConfig(),
		GateConfig:   DefaultGateConfig(),
	}
}

// NewDaemon creates a daemon that grades agentrace events.
func NewDaemon(cfg DaemonConfig) *Daemon {
	return &Daemon{
		cfg:     cfg,
		graders: AllGraders(cfg.GraderConfig),
		done:    make(chan struct{}),
	}
}

// Start begins the daemon's background processing loop.
// Call Stop() to shut down gracefully.
func (d *Daemon) Start(ctx context.Context) error {
	ctx, d.cancel = context.WithCancel(ctx)
	go d.runLoop(ctx)
	return nil
}

// Stop signals the daemon to shut down.
func (d *Daemon) Stop() {
	if d.cancel != nil {
		d.cancel()
	}
	<-d.done
}

// Results returns all grade results collected so far.
func (d *Daemon) Results() []GradeResult {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]GradeResult, len(d.results))
	copy(out, d.results)
	return out
}

// GateCheck runs the quality gate against collected results' source events.
func (d *Daemon) GateCheck(events []AgentTraceEvent) GateVerdict {
	return EvaluateGate(events, d.graders, d.cfg.GateConfig)
}

// GradeEvent processes a single event through all graders.
func (d *Daemon) GradeEvent(event AgentTraceEvent) []GradeResult {
	var results []GradeResult
	for _, g := range d.graders {
		result := g.Grade(event)
		results = append(results, result)
	}
	d.mu.Lock()
	d.results = append(d.results, results...)
	d.mu.Unlock()
	return results
}

// TailNDJSON reads agentrace NDJSON from a reader and grades each event.
// Blocks until the reader is exhausted or context is cancelled.
func (d *Daemon) TailNDJSON(ctx context.Context, r io.Reader) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var event AgentTraceEvent
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}
		d.GradeEvent(event)
	}
	return scanner.Err()
}

// HealthStatus returns a structured health check for the daemon.
type HealthStatus struct {
	Running       bool   `json:"running"`
	EventsGraded  int    `json:"events_graded"`
	LastGradeAt   string `json:"last_grade_at,omitempty"`
	UptimeSeconds int64  `json:"uptime_seconds"`
}

// Health returns current daemon health.
func (d *Daemon) Health() HealthStatus {
	d.mu.RLock()
	defer d.mu.RUnlock()
	status := HealthStatus{
		Running:      d.cancel != nil,
		EventsGraded: len(d.results),
	}
	if len(d.results) > 0 {
		status.LastGradeAt = d.results[len(d.results)-1].Timestamp
	}
	return status
}

func (d *Daemon) runLoop(ctx context.Context) {
	defer close(d.done)

	if d.cfg.AgentracePath == "" {
		return
	}

	ticker := time.NewTicker(d.cfg.PollInterval)
	defer ticker.Stop()

	var lastSize int64
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.pollFile(ctx, &lastSize)
		}
	}
}

func (d *Daemon) pollFile(ctx context.Context, lastSize *int64) {
	info, err := os.Stat(d.cfg.AgentracePath)
	if err != nil {
		return
	}
	currentSize := info.Size()
	if currentSize <= *lastSize {
		return
	}

	f, err := os.Open(d.cfg.AgentracePath)
	if err != nil {
		return
	}
	defer f.Close()

	if _, err := f.Seek(*lastSize, io.SeekStart); err != nil {
		return
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var event AgentTraceEvent
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}
		d.GradeEvent(event)
	}

	newInfo, err := os.Stat(d.cfg.AgentracePath)
	if err == nil {
		*lastSize = newInfo.Size()
	}
}

// FormatHealthJSON returns health as a JSON string.
func (d *Daemon) FormatHealthJSON() string {
	h := d.Health()
	b, err := json.Marshal(h)
	if err != nil {
		return fmt.Sprintf(`{"error":"%s"}`, err.Error())
	}
	return string(b)
}
