package main

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	eventsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agentrace_events_total",
			Help: "Total agentrace events by type",
		},
		[]string{"event_type", "agent_id"},
	)
	eventLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "agentrace_event_latency_seconds",
			Help:    "Agentrace event processing latency",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"event_type"},
	)
	agentActive = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "agentrace_agent_active",
			Help: "Whether an agent is currently active",
		},
		[]string{"agent_id"},
	)
	tailPosition = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "agentrace_tail_position_bytes",
			Help: "Current byte offset in the tailed NDJSON file",
		},
	)
)

func init() {
	prometheus.MustRegister(eventsTotal, eventLatency, agentActive, tailPosition)
}

type traceEvent struct {
	Timestamp string  `json:"ts"`
	Event     string  `json:"event"`
	AgentID   string  `json:"agent_id"`
	LatencyMS float64 `json:"latency_ms"`
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logPath := getEnv("AGENTRACE_LOG_PATH", os.ExpandEnv("$HOME/logs/runx/agentrace-mcp.ndjson"))
	go tailLoop(ctx, logPath)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	addr := getEnv("AGENTRACE_BRIDGE_ADDR", ":9101")
	srv := &http.Server{Addr: addr, Handler: mux}

	go func() {
		<-ctx.Done()
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutCancel()
		srv.Close()
		_ = shutCtx
	}()

	slog.Info("agentrace-bridge starting", "addr", addr, "log_path", logPath)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
	slog.Info("agentrace-bridge stopped")
}

func tailLoop(ctx context.Context, path string) {
	for {
		if err := tailFile(ctx, path); err != nil {
			slog.Warn("tail error, retrying in 5s", "error", err, "path", path)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func tailFile(ctx context.Context, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		return err
	}

	scanner := bufio.NewScanner(f)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if scanner.Scan() {
			processLine(scanner.Bytes())
			pos, _ := f.Seek(0, io.SeekCurrent)
			tailPosition.Set(float64(pos))
		} else {
			if err := scanner.Err(); err != nil {
				return err
			}
			time.Sleep(500 * time.Millisecond)
			scanner = bufio.NewScanner(f)
		}
	}
}

func processLine(line []byte) {
	var ev traceEvent
	if err := json.Unmarshal(line, &ev); err != nil {
		return
	}
	if ev.Event == "" {
		return
	}

	agentID := ev.AgentID
	if agentID == "" {
		agentID = "unknown"
	}

	eventsTotal.WithLabelValues(ev.Event, agentID).Inc()

	if ev.LatencyMS > 0 {
		eventLatency.WithLabelValues(ev.Event).Observe(ev.LatencyMS / 1000.0)
	}

	agentActive.WithLabelValues(agentID).Set(1)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
