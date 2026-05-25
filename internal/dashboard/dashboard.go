package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Server is the HTTP dashboard backed by a set of Fetchers.
type Server struct {
	Fetchers       []Fetcher
	ManifestPath   string // YAML roadmap manifest path
	ListenAddr     string
	AuthToken      string // if set, requires Bearer token on all routes except /healthz
	pages          map[string]*template.Template
	mu             sync.RWMutex
	cachedStatuses []namedStatus
}

type namedStatus struct {
	Name   string
	Status Status
}

// PageData is the common template data.
type PageData struct {
	Title       string
	RefreshedAt string
	Statuses    []namedStatus
	// Page-specific fields below
	Pipelines   map[int][]GitLabPipeline
	GHWorkflows map[string][]GitHubWorkflowRun
	Agents      []SprintBoardAgent
	Sprints     []sprintRow
	Nodes       []fleetNode
	Components  []roadmapComponent
}

type sprintRow struct {
	Sprint      SprintBoardSprint
	TicketCount int
	DoneCount   int
}

type fleetNode struct {
	Name      string
	Level     string
	LastProbe string
	FreePct   int
	Services  []fleetService
}

type fleetService struct {
	Name   string
	Status string // "up", "down", "unknown"
}

type roadmapComponent struct {
	Name   string `yaml:"name"`
	Status string `yaml:"status"`
	Pct    int    `yaml:"pct"`
}

type roadmapManifest struct {
	Components []roadmapComponent `yaml:"components"`
}

// New creates a Server with parsed templates. Call ListenAndServe to start.
func New(fetchers []Fetcher, manifestPath, listenAddr string) (*Server, error) {
	funcMap := template.FuncMap{
		"ge": func(a, b int) bool { return a >= b },
	}

	layoutTmpl, err := template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/layout.html")
	if err != nil {
		return nil, fmt.Errorf("parse layout: %w", err)
	}

	pageFiles := []string{"overview.html", "ci.html", "agents.html", "sprints.html", "fleet.html", "roadmap.html"}
	pages := make(map[string]*template.Template, len(pageFiles))
	for _, pf := range pageFiles {
		clone, err := layoutTmpl.Clone()
		if err != nil {
			return nil, fmt.Errorf("clone layout for %s: %w", pf, err)
		}
		_, err = clone.ParseFS(templateFS, "templates/"+pf)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", pf, err)
		}
		pages[pf] = clone
	}

	return &Server{
		Fetchers:     fetchers,
		ManifestPath: manifestPath,
		ListenAddr:   listenAddr,
		AuthToken:    os.Getenv("DASHBOARD_TOKEN"),
		pages:        pages,
	}, nil
}

// Handler returns the http.Handler with all routes registered.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleOverview)
	mux.HandleFunc("/ci", s.handleCI)
	mux.HandleFunc("/agents", s.handleAgents)
	mux.HandleFunc("/sprints", s.handleSprints)
	mux.HandleFunc("/fleet", s.handleFleet)
	mux.HandleFunc("/roadmap", s.handleRoadmap)
	mux.HandleFunc("/api/health", s.handleAPIHealth)
	mux.HandleFunc("/healthz", s.handleAPIHealth)

	if s.AuthToken != "" {
		return s.authMiddleware(mux)
	}
	return mux
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer "+s.AuthToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	log.Printf("dashboard listening on %s", s.ListenAddr)
	return http.ListenAndServe(s.ListenAddr, s.Handler())
}

func (s *Server) refreshAll(ctx context.Context) []namedStatus {
	results := make([]namedStatus, len(s.Fetchers))
	var wg sync.WaitGroup
	for i, f := range s.Fetchers {
		wg.Add(1)
		go func(idx int, fetcher Fetcher) {
			defer wg.Done()
			st, err := fetcher.Fetch(ctx)
			if err != nil {
				st = Status{Level: "RED", Message: fmt.Sprintf("fetch error: %v", err)}
			}
			results[idx] = namedStatus{Name: fetcher.Name(), Status: st}
		}(i, f)
	}
	wg.Wait()

	s.mu.Lock()
	s.cachedStatuses = results
	s.mu.Unlock()
	return results
}

func (s *Server) getStatuses(ctx context.Context) []namedStatus {
	s.mu.RLock()
	cached := s.cachedStatuses
	s.mu.RUnlock()
	if cached != nil {
		return cached
	}
	return s.refreshAll(ctx)
}

func (s *Server) render(w http.ResponseWriter, name string, data PageData) {
	data.RefreshedAt = time.Now().Format(time.RFC3339)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl, ok := s.pages[name]
	if !ok {
		http.Error(w, "unknown page: "+name, http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	statuses := s.refreshAll(r.Context())
	s.render(w, "overview.html", PageData{
		Title:    "Overview",
		Statuses: statuses,
	})
}

func (s *Server) handleCI(w http.ResponseWriter, r *http.Request) {
	statuses := s.getStatuses(r.Context())
	pipelines := make(map[int][]GitLabPipeline)
	ghWorkflows := make(map[string][]GitHubWorkflowRun)
	for _, ns := range statuses {
		if ns.Name == "gitlab" {
			if data, ok := ns.Status.Data.(map[int][]GitLabPipeline); ok {
				pipelines = data
			}
		}
		if ns.Name == "github-ci" {
			if data, ok := ns.Status.Data.(map[string][]GitHubWorkflowRun); ok {
				ghWorkflows = data
			}
		}
	}
	s.render(w, "ci.html", PageData{Title: "CI Pipelines", Pipelines: pipelines, GHWorkflows: ghWorkflows})
}

func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
	statuses := s.getStatuses(r.Context())
	var agents []SprintBoardAgent
	for _, ns := range statuses {
		if ns.Name == "sprintboard" {
			if data, ok := ns.Status.Data.(map[string]interface{}); ok {
				if a, ok := data["agents"].([]SprintBoardAgent); ok {
					agents = a
				}
			}
		}
	}
	s.render(w, "agents.html", PageData{Title: "Agents", Agents: agents})
}

func (s *Server) handleSprints(w http.ResponseWriter, r *http.Request) {
	statuses := s.getStatuses(r.Context())
	var rows []sprintRow
	var allTickets []SprintBoardTicket
	var allSprints []SprintBoardSprint
	var agents []SprintBoardAgent

	for _, ns := range statuses {
		if ns.Name == "sprintboard" {
			if data, ok := ns.Status.Data.(map[string]interface{}); ok {
				if t, ok := data["tickets"].([]SprintBoardTicket); ok {
					allTickets = t
				}
				if sp, ok := data["sprints"].([]SprintBoardSprint); ok {
					allSprints = sp
				}
				if a, ok := data["agents"].([]SprintBoardAgent); ok {
					agents = a
				}
			}
		}
	}
	for _, sp := range allSprints {
		total, done := 0, 0
		for _, t := range allTickets {
			if t.SprintID == sp.ID {
				total++
				if t.Status == "done" || t.Status == "completed" {
					done++
				}
			}
		}
		rows = append(rows, sprintRow{Sprint: sp, TicketCount: total, DoneCount: done})
	}
	s.render(w, "sprints.html", PageData{Title: "Sprints", Sprints: rows, Agents: agents})
}

func (s *Server) handleFleet(w http.ResponseWriter, r *http.Request) {
	nodes := s.readFleetProbe()
	s.render(w, "fleet.html", PageData{Title: "Fleet", Nodes: nodes})
}

func (s *Server) readFleetProbe() []fleetNode {
	home, _ := os.UserHomeDir()
	path := home + "/logs/runx/resource-probe.ndjson"
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	events, _ := ParseNDJSONReader(f, 0)
	seen := map[string]fleetNode{}
	for _, e := range events {
		var probe struct {
			Ts       string         `json:"ts"`
			FreePct  int            `json:"free_pct"`
			Host     string         `json:"host"`
			Services []serviceProbe `json:"services,omitempty"`
		}
		raw, _ := json.Marshal(e)
		_ = json.Unmarshal(raw, &probe)
		if probe.Host == "" {
			probe.Host = "macbook"
		}
		level := "GREEN"
		if probe.FreePct < 5 {
			level = "RED"
		} else if probe.FreePct < 15 {
			level = "YELLOW"
		}
		var services []fleetService
		for _, svc := range probe.Services {
			services = append(services, fleetService{Name: svc.Name, Status: svc.Status})
			if svc.Status == "down" && level != "RED" {
				level = "YELLOW"
			}
		}
		seen[probe.Host] = fleetNode{
			Name:      probe.Host,
			Level:     level,
			LastProbe: probe.Ts,
			FreePct:   probe.FreePct,
			Services:  services,
		}
	}
	nodes := make([]fleetNode, 0, len(seen))
	for _, n := range seen {
		nodes = append(nodes, n)
	}
	return nodes
}

type serviceProbe struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

func (s *Server) handleRoadmap(w http.ResponseWriter, r *http.Request) {
	var components []roadmapComponent
	if s.ManifestPath != "" {
		data, err := os.ReadFile(s.ManifestPath)
		if err == nil {
			var m roadmapManifest
			if yaml.Unmarshal(data, &m) == nil {
				components = m.Components
			}
		}
	}
	s.render(w, "roadmap.html", PageData{Title: "Roadmap", Components: components})
}

func (s *Server) handleAPIHealth(w http.ResponseWriter, r *http.Request) {
	statuses := s.refreshAll(r.Context())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(statuses)
}
