package fleetagent

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"
)

// HealthServer provides a /healthz endpoint for the fleet agent.
type HealthServer struct {
	agent     *Agent
	startTime time.Time
	pollCount atomic.Int64
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

type healthResponse struct {
	Status    string `json:"status"`
	AgentID   string `json:"agent_id"`
	UptimeSec int64  `json:"uptime_seconds"`
	PollCount int64  `json:"poll_count"`
}

// Handler returns an http.Handler for the /healthz endpoint.
func (h *HealthServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		resp := healthResponse{
			Status:    "ok",
			AgentID:   h.agent.cfg.AgentID,
			UptimeSec: int64(time.Since(h.startTime).Seconds()),
			PollCount: h.pollCount.Load(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	return mux
}
