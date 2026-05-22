package mem0watchdog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type Config struct {
	ProbeURL      string
	ProbeInterval time.Duration
	ProbeTimeout  time.Duration
	FailThreshold int
	LogPath       string
	PortPattern   string
}

func DefaultConfig() Config {
	return Config{
		ProbeURL:      "http://127.0.0.1:18888/docs",
		ProbeInterval: 60 * time.Second,
		ProbeTimeout:  10 * time.Second,
		FailThreshold: 3,
		LogPath:       os.ExpandEnv("$HOME/logs/runx/mem0-watchdog.ndjson"),
		PortPattern:   "18888",
	}
}

type Watchdog struct {
	cfg              Config
	consecutiveFails int
	mu               sync.Mutex
	client           *http.Client
}

func New(cfg Config) *Watchdog {
	return &Watchdog{
		cfg:    cfg,
		client: &http.Client{Timeout: cfg.ProbeTimeout},
	}
}

func (w *Watchdog) Probe(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, w.cfg.ProbeURL, nil)
	if err != nil {
		return err
	}
	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("probe returned %d", resp.StatusCode)
	}
	return nil
}

func (w *Watchdog) KillStaleTunnels() ([]int, error) {
	out, err := exec.Command("pgrep", "-fl", fmt.Sprintf("ssh.*%s", w.cfg.PortPattern)).Output()
	if err != nil {
		return nil, nil
	}
	var killed []int
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		var pid int
		fmt.Sscanf(line, "%d", &pid)
		if pid > 0 {
			if err := exec.Command("kill", "-9", fmt.Sprintf("%d", pid)).Run(); err == nil {
				killed = append(killed, pid)
			}
		}
	}
	return killed, nil
}

type LogEntry struct {
	Timestamp string `json:"ts"`
	Event     string `json:"event"`
	Status    string `json:"status"`
	Fails     int    `json:"consecutive_fails,omitempty"`
	Killed    []int  `json:"killed_pids,omitempty"`
	Error     string `json:"error,omitempty"`
}

func (w *Watchdog) log(entry LogEntry) {
	entry.Timestamp = time.Now().Format(time.RFC3339)
	f, err := os.OpenFile(w.cfg.LogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	json.NewEncoder(f).Encode(entry)
}

func (w *Watchdog) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.cfg.ProbeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := w.Probe(ctx); err != nil {
				w.mu.Lock()
				w.consecutiveFails++
				fails := w.consecutiveFails
				w.mu.Unlock()

				w.log(LogEntry{Event: "probe_fail", Status: "fail", Fails: fails, Error: err.Error()})

				if fails >= w.cfg.FailThreshold {
					killed, _ := w.KillStaleTunnels()
					w.log(LogEntry{Event: "tunnel_restart", Status: "killed", Killed: killed})
					w.mu.Lock()
					w.consecutiveFails = 0
					w.mu.Unlock()
				}
			} else {
				w.mu.Lock()
				w.consecutiveFails = 0
				w.mu.Unlock()
				w.log(LogEntry{Event: "probe_ok", Status: "ok"})
			}
		}
	}
}
