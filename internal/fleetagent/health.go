package fleetagent

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"
)

// HealthServer provides /healthz and /readyz endpoints for the fleet agent.
type HealthServer struct {
	agent     *Agent
	startTime time.Time
	pollCount atomic.Int64
	shutting  atomic.Bool
}

// NewHealthServer creates a health server bound to the given agent.
func NewHealthServer(agent *Agent) *HealthServer {
	return &HealthServer{
		agent:     agent,
		startTime: time.Now(),
	}
}

// IncrementPollCount records a poll cycle (call from the agent loop).
func (h *HealthServer) IncrementPollCount() {
	h.pollCount.Add(1)
}

// SetShuttingDown marks the server as not ready to accept work.
func (h *HealthServer) SetShuttingDown() {
	h.shutting.Store(true)
}

type healthResponse struct {
	Status    string `json:"status"`
	AgentID   string `json:"agent_id"`
	UptimeSec int64  `json:"uptime_seconds"`
	PollCount int64  `json:"poll_count"`
}

// Handler returns an http.Handler for the /healthz and /readyz endpoints.
func (h *HealthServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", h.handleHealthz)
	mux.HandleFunc("/readyz", h.handleReadyz)
	return mux
}

func (h *HealthServer) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	resp := healthResponse{
		Status:    "ok",
		AgentID:   h.agent.cfg.AgentID,
		UptimeSec: int64(time.Since(h.startTime).Seconds()),
		PollCount: h.pollCount.Load(),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *HealthServer) handleReadyz(w http.ResponseWriter, _ *http.Request) {
	if h.shutting.Load() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"status": "shutting_down"})
		return
	}
	resp := healthResponse{
		Status:    "ok",
		AgentID:   h.agent.cfg.AgentID,
		UptimeSec: int64(time.Since(h.startTime).Seconds()),
		PollCount: h.pollCount.Load(),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
