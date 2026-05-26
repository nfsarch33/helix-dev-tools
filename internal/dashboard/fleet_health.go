package dashboard

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// FleetHealthResponse is the JSON shape for GET /api/fleet/health.
type FleetHealthResponse struct {
	Timestamp string             `json:"timestamp"`
	Probes    []FleetHealthProbe `json:"probes"`
	Summary   FleetHealthSummary `json:"summary"`
}

// FleetHealthProbe is one service probe result.
type FleetHealthProbe struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	LatencyMs int64  `json:"latency_ms"`
}

// FleetHealthSummary counts probes by status level.
type FleetHealthSummary struct {
	Green  int `json:"green"`
	Yellow int `json:"yellow"`
	Red    int `json:"red"`
}

type fleetHealthCache struct {
	mu        sync.RWMutex
	response  *FleetHealthResponse
	expiresAt time.Time
}

func (c *fleetHealthCache) get() *FleetHealthResponse {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.response != nil && time.Now().Before(c.expiresAt) {
		return c.response
	}
	return nil
}

func (c *fleetHealthCache) set(resp *FleetHealthResponse, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.response = resp
	c.expiresAt = time.Now().Add(ttl)
}

const fleetHealthCacheTTL = 60 * time.Second

func (s *Server) handleFleetHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if cached := s.fleetHealthCache.get(); cached != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cached)
		return
	}

	start := time.Now()
	nodes := s.readFleetProbe()
	elapsed := time.Since(start)

	resp := buildFleetHealthResponse(nodes, elapsed)
	s.fleetHealthCache.set(&resp, fleetHealthCacheTTL)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func buildFleetHealthResponse(nodes []fleetNode, readLatency time.Duration) FleetHealthResponse {
	var probes []FleetHealthProbe
	summary := FleetHealthSummary{}

	baseLatency := readLatency.Milliseconds()
	if baseLatency < 1 {
		baseLatency = 1
	}

	for _, node := range nodes {
		status := statusFromLevel(node.Level)
		probes = append(probes, FleetHealthProbe{
			Name:      node.Name,
			Status:    status,
			LatencyMs: baseLatency,
		})
		countStatus(&summary, status)

		for _, svc := range node.Services {
			svcStatus := statusFromServiceStatus(svc.Status)
			probes = append(probes, FleetHealthProbe{
				Name:      node.Name + "/" + svc.Name,
				Status:    svcStatus,
				LatencyMs: baseLatency,
			})
			countStatus(&summary, svcStatus)
		}
	}

	if len(probes) == 0 {
		probes = []FleetHealthProbe{}
	}

	return FleetHealthResponse{
		Timestamp: time.Now().Format("2006-01-02T15:04-07:00"),
		Probes:    probes,
		Summary:   summary,
	}
}

func statusFromLevel(level string) string {
	switch level {
	case "GREEN":
		return "green"
	case "YELLOW":
		return "yellow"
	case "RED":
		return "red"
	default:
		return "red"
	}
}

func statusFromServiceStatus(s string) string {
	switch s {
	case "up":
		return "green"
	case "down":
		return "red"
	default:
		return "yellow"
	}
}

func countStatus(s *FleetHealthSummary, status string) {
	switch status {
	case "green":
		s.Green++
	case "yellow":
		s.Yellow++
	case "red":
		s.Red++
	}
}
