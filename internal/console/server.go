package console

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// HealthStatus represents the health of a single subsystem
type HealthStatus struct {
	Name    string `json:"name"`
	Healthy bool   `json:"healthy"`
	Detail  string `json:"detail,omitempty"`
}

// DashboardData is the top-level console response
type DashboardData struct {
	Timestamp  time.Time      `json:"timestamp"`
	Components []HealthStatus `json:"components"`
	AgentCount int            `json:"agent_count"`
	SprintID   string         `json:"sprint_id,omitempty"`
}

// HealthProbe is a function that checks one subsystem
type HealthProbe func() HealthStatus

// Server is the operator console HTTP server
type Server struct {
	mux    *http.ServeMux
	probes []HealthProbe
}

// NewServer creates a console server with the given health probes
func NewServer(probes []HealthProbe) *Server {
	s := &Server{
		mux:    http.NewServeMux(),
		probes: probes,
	}
	s.mux.HandleFunc("/healthz", s.handleHealthz)
	s.mux.HandleFunc("/api/dashboard", s.handleDashboard)
	s.mux.HandleFunc("/api/components", s.handleComponents)
	return s
}

// Handler returns the HTTP handler for the console
func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ok")
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	data := DashboardData{
		Timestamp:  time.Now(),
		Components: s.runProbes(),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (s *Server) handleComponents(w http.ResponseWriter, r *http.Request) {
	components := s.runProbes()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(components)
}

func (s *Server) runProbes() []HealthStatus {
	var results []HealthStatus
	for _, probe := range s.probes {
		results = append(results, probe())
	}
	return results
}
