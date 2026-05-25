package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DaemonConfig controls the eval daemon scheduler.
type DaemonConfig struct {
	EvalDir    string
	Interval   time.Duration
	NDJSONPath string
	StorePath  string
	RunOnStart bool
}

// Daemon runs eval harness on a schedule and emits NDJSON results.
type Daemon struct {
	cfg      DaemonConfig
	store    *EvalStore
	log      *slog.Logger
	mu       sync.Mutex
	lastRun  time.Time
	runCount int
}

// NewDaemon creates a scheduled eval runner.
func NewDaemon(cfg DaemonConfig, log *slog.Logger) (*Daemon, error) {
	if cfg.Interval <= 0 {
		cfg.Interval = 1 * time.Hour
	}
	if cfg.NDJSONPath == "" {
		home, _ := os.UserHomeDir()
		cfg.NDJSONPath = filepath.Join(home, "logs", "eval-results.ndjson")
	}
	if cfg.StorePath == "" {
		cfg.StorePath = DefaultEvalDBPath()
	}
	if log == nil {
		log = slog.Default()
	}

	store, err := OpenEvalStore(cfg.StorePath)
	if err != nil {
		return nil, fmt.Errorf("open eval store: %w", err)
	}

	return &Daemon{cfg: cfg, store: store, log: log}, nil
}

// Run starts the daemon loop. It blocks until ctx is cancelled.
func (d *Daemon) Run(ctx context.Context) error {
	d.log.Info("eval daemon starting",
		"interval", d.cfg.Interval,
		"eval_dir", d.cfg.EvalDir,
		"ndjson", d.cfg.NDJSONPath,
	)

	if d.cfg.RunOnStart {
		d.runCycle(ctx)
	}

	ticker := time.NewTicker(d.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			d.log.Info("eval daemon shutting down", "runs", d.runCount)
			d.store.Close()
			return ctx.Err()
		case <-ticker.C:
			d.runCycle(ctx)
		}
	}
}

// RunOnce executes a single eval cycle (useful for testing and CLI).
func (d *Daemon) RunOnce(ctx context.Context) (Report, error) {
	return d.runCycle(ctx), nil
}

// Stats returns daemon runtime stats.
func (d *Daemon) Stats() DaemonStats {
	d.mu.Lock()
	defer d.mu.Unlock()
	return DaemonStats{
		RunCount: d.runCount,
		LastRun:  d.lastRun,
	}
}

// Close releases daemon resources.
func (d *Daemon) Close() error {
	return d.store.Close()
}

// DaemonStats contains runtime statistics.
type DaemonStats struct {
	RunCount int       `json:"run_count"`
	LastRun  time.Time `json:"last_run"`
}

func (d *Daemon) runCycle(ctx context.Context) Report {
	d.log.Info("eval cycle starting")
	start := time.Now()

	results, err := RunAllEvalsInDir(d.cfg.EvalDir)
	if err != nil {
		d.log.Error("eval cycle failed", "error", err)
		return Report{}
	}

	runID := fmt.Sprintf("daemon-%s", start.Format("20060102T150405"))
	report := GenerateReport(runID, results)

	for _, r := range results {
		if storeErr := d.store.SaveResult(runID, r); storeErr != nil {
			d.log.Error("save result failed", "eval_id", r.EvalID, "error", storeErr)
		}
	}

	if writeErr := d.writeNDJSON(results, runID); writeErr != nil {
		d.log.Error("ndjson write failed", "error", writeErr)
	}

	d.mu.Lock()
	d.runCount++
	d.lastRun = start
	d.mu.Unlock()

	d.log.Info("eval cycle complete",
		"run_id", runID,
		"evals", len(results),
		"pass_rate", fmt.Sprintf("%.1f%%", report.PassRate*100),
		"duration", time.Since(start).Round(time.Millisecond),
	)

	_ = ctx
	return report
}

// NDJSONEvalEntry is a single eval result NDJSON line.
type NDJSONEvalEntry struct {
	Timestamp  time.Time `json:"ts"`
	RunID      string    `json:"run_id"`
	EvalID     string    `json:"eval_id"`
	EvalName   string    `json:"eval_name"`
	EvalType   EvalType  `json:"eval_type"`
	Pass       bool      `json:"pass"`
	Score      float64   `json:"score"`
	DurationMS int64     `json:"duration_ms"`
	Iterations int       `json:"iterations"`
	Error      string    `json:"error,omitempty"`
}

func (d *Daemon) writeNDJSON(results []EvalResult, runID string) error {
	dir := filepath.Dir(d.cfg.NDJSONPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create ndjson dir: %w", err)
	}

	f, err := os.OpenFile(d.cfg.NDJSONPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("open ndjson: %w", err)
	}
	defer f.Close()

	for _, r := range results {
		entry := NDJSONEvalEntry{
			Timestamp:  r.Timestamp,
			RunID:      runID,
			EvalID:     r.EvalID,
			EvalName:   r.EvalName,
			EvalType:   r.EvalType,
			Pass:       r.Pass,
			Score:      r.Score,
			DurationMS: r.DurationMS,
			Iterations: r.Iterations,
			Error:      r.Error,
		}
		line, err := json.Marshal(entry)
		if err != nil {
			continue
		}
		f.Write(append(line, '\n'))
	}
	return nil
}
