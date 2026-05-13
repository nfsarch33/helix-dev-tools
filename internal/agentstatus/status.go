package agentstatus

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"log/slog"
)

// AgentState represents the operational state of an agent.
type AgentState string

const (
	StateIdle    AgentState = "idle"
	StateRunning AgentState = "running"
	StatePaused  AgentState = "paused"
	StateErrored AgentState = "errored"
	StateHung    AgentState = "hung"
)

// AgentStatus is a point-in-time snapshot of one agent's state.
type AgentStatus struct {
	Name         string            `json:"name"`
	State        AgentState        `json:"state"`
	LastActivity time.Time         `json:"last_activity"`
	RunID        string            `json:"run_id,omitempty"`
	Uptime       time.Duration     `json:"uptime_ms"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// StatusReport aggregates all known agent statuses.
type StatusReport struct {
	Timestamp     time.Time     `json:"timestamp"`
	Agents        []AgentStatus `json:"agents"`
	MemoryFreePct int           `json:"memory_free_pct"`
}

// Source reads agent state from a data source (agentrace, resource-probe, etc.).
type Source interface {
	Name() string
	Read(ctx context.Context) ([]AgentStatus, error)
}

// Collector aggregates multiple Sources into a StatusReport.
type Collector struct {
	mu            sync.RWMutex
	sources       []Source
	logger        *slog.Logger
	hangThreshold time.Duration
}

// NewCollector creates a Collector with the given sources.
func NewCollector(logger *slog.Logger, hangThreshold time.Duration, sources ...Source) *Collector {
	if hangThreshold <= 0 {
		hangThreshold = 15 * time.Minute
	}
	return &Collector{
		sources:       sources,
		logger:        logger,
		hangThreshold: hangThreshold,
	}
}

// Collect gathers status from all sources and produces a report.
func (c *Collector) Collect(ctx context.Context) StatusReport {
	report := StatusReport{Timestamp: time.Now()}

	for _, src := range c.sources {
		statuses, err := src.Read(ctx)
		if err != nil {
			c.logger.Warn("source read failed", "source", src.Name(), "error", err)
			continue
		}
		report.Agents = append(report.Agents, statuses...)
	}

	now := time.Now()
	for i := range report.Agents {
		if report.Agents[i].State == StateRunning && now.Sub(report.Agents[i].LastActivity) > c.hangThreshold {
			report.Agents[i].State = StateHung
		}
	}

	report.MemoryFreePct = readMemoryFreePct()
	return report
}

func readMemoryFreePct() int {
	home, err := os.UserHomeDir()
	if err != nil {
		return -1
	}
	path := filepath.Join(home, "logs", "runx", "resource-probe.ndjson")
	f, err := os.Open(path)
	if err != nil {
		return -1
	}
	defer f.Close()

	var lastLine string
	buf := make([]byte, 4096)
	stat, _ := f.Stat()
	offset := stat.Size() - int64(len(buf))
	if offset < 0 {
		offset = 0
	}
	if _, err := f.ReadAt(buf, offset); err != nil && err != io.EOF {
		return -1
	}
	lines := strings.Split(string(buf), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			lastLine = strings.TrimSpace(lines[i])
			break
		}
	}
	if lastLine == "" {
		return -1
	}

	var probe struct {
		FreePct int `json:"free_pct"`
	}
	if err := json.Unmarshal([]byte(lastLine), &probe); err != nil {
		return -1
	}
	return probe.FreePct
}

// Handler returns an http.Handler that serves the status report as JSON.
func (c *Collector) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		report := c.Collect(r.Context())
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(report)
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	return mux
}

// Serve starts the Agent Status API on the given address (e.g. "127.0.0.1:9195").
// It blocks until the context is cancelled.
func (c *Collector) Serve(ctx context.Context, addr string) error {
	srv := &http.Server{Addr: addr, Handler: c.Handler()}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	c.logger.Info("agent status API starting", "addr", addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}
