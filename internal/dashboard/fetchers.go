package dashboard

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	_ "modernc.org/sqlite"
)

// Status represents the health result from a single fetcher.
type Status struct {
	Level   string      `json:"level"`   // "GREEN", "YELLOW", "RED"
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Fetcher is the interface every subsystem health-check implements.
type Fetcher interface {
	Name() string
	Fetch(ctx context.Context) (Status, error)
}

// --- GitLab ---

// GitLabPipeline is one row from the GitLab pipelines API.
type GitLabPipeline struct {
	ID        int    `json:"id"`
	Status    string `json:"status"`
	Ref       string `json:"ref"`
	CreatedAt string `json:"created_at"`
	WebURL    string `json:"web_url"`
}

// GitLabFetcher queries the GitLab v4 pipelines API.
type GitLabFetcher struct {
	BaseURL    string // e.g. "http://localhost:30080"
	ProjectIDs []int
	Token      string
	Client     *http.Client
}

func (f *GitLabFetcher) Name() string { return "gitlab" }

func (f *GitLabFetcher) Fetch(ctx context.Context) (Status, error) {
	client := f.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	allPipelines := make(map[int][]GitLabPipeline)
	for _, pid := range f.ProjectIDs {
		url := fmt.Sprintf("%s/api/v4/projects/%d/pipelines?per_page=10", f.BaseURL, pid)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return Status{Level: "RED", Message: fmt.Sprintf("request build: %v", err)}, err
		}
		if f.Token != "" {
			req.Header.Set("PRIVATE-TOKEN", f.Token)
		}
		resp, err := client.Do(req)
		if err != nil {
			return Status{Level: "RED", Message: fmt.Sprintf("gitlab unreachable: %v", err)}, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return Status{Level: "RED", Message: fmt.Sprintf("gitlab HTTP %d for project %d", resp.StatusCode, pid)}, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		var pipelines []GitLabPipeline
		if err := json.NewDecoder(resp.Body).Decode(&pipelines); err != nil {
			return Status{Level: "RED", Message: fmt.Sprintf("decode: %v", err)}, err
		}
		allPipelines[pid] = pipelines
	}

	level := "GREEN"
	for _, pipes := range allPipelines {
		for _, p := range pipes {
			if p.Status == "failed" {
				level = "RED"
				break
			}
			if p.Status == "running" || p.Status == "pending" {
				level = "YELLOW"
			}
		}
	}
	return Status{Level: level, Message: "pipelines fetched", Data: allPipelines}, nil
}

// --- ArgoCD ---

// ArgoCDApp is a simplified ArgoCD application status.
type ArgoCDApp struct {
	Name       string `json:"name"`
	SyncStatus string `json:"sync_status"`
	Health     string `json:"health"`
}

// ArgoCDFetcher queries the ArgoCD API for application health.
type ArgoCDFetcher struct {
	BaseURL string // e.g. "http://localhost:30880"
	Token   string
	Client  *http.Client
}

func (f *ArgoCDFetcher) Name() string { return "argocd" }

func (f *ArgoCDFetcher) Fetch(ctx context.Context) (Status, error) {
	client := f.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	url := fmt.Sprintf("%s/api/v1/applications", f.BaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Status{Level: "RED", Message: fmt.Sprintf("request build: %v", err)}, err
	}
	if f.Token != "" {
		req.Header.Set("Authorization", "Bearer "+f.Token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return Status{Level: "RED", Message: fmt.Sprintf("argocd unreachable: %v", err)}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Status{Level: "RED", Message: fmt.Sprintf("argocd HTTP %d", resp.StatusCode)}, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var body struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Status struct {
				Sync struct {
					Status string `json:"status"`
				} `json:"sync"`
				Health struct {
					Status string `json:"status"`
				} `json:"health"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Status{Level: "RED", Message: fmt.Sprintf("decode: %v", err)}, err
	}

	apps := make([]ArgoCDApp, 0, len(body.Items))
	level := "GREEN"
	for _, item := range body.Items {
		a := ArgoCDApp{
			Name:       item.Metadata.Name,
			SyncStatus: item.Status.Sync.Status,
			Health:     item.Status.Health.Status,
		}
		apps = append(apps, a)
		if a.Health == "Degraded" || a.Health == "Missing" {
			level = "RED"
		} else if a.SyncStatus != "Synced" && level != "RED" {
			level = "YELLOW"
		}
	}
	return Status{Level: level, Message: fmt.Sprintf("%d apps", len(apps)), Data: apps}, nil
}

// --- SprintBoard ---

// SprintBoardAgent is a registered agent row.
type SprintBoardAgent struct {
	ID              string `json:"id"`
	Surface         string `json:"surface"`
	CurrentTicketID string `json:"current_ticket_id"`
	LastSeen        string `json:"last_seen"`
}

// SprintBoardTicket is a ticket row.
type SprintBoardTicket struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	ClaimedBy string `json:"claimed_by"`
	SprintID  string `json:"sprint_id"`
}

// SprintBoardSprint is a sprint row.
type SprintBoardSprint struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// SprintBoardFetcher reads the local SQLite sprintboard database.
type SprintBoardFetcher struct {
	DBPath string // defaults to ~/.config/helix-dev-tools/sprintboard.db
	db     *sql.DB
}

func (f *SprintBoardFetcher) Name() string { return "sprintboard" }

func (f *SprintBoardFetcher) openDB() (*sql.DB, error) {
	if f.db != nil {
		return f.db, nil
	}
	dbPath := f.DBPath
	if dbPath == "" {
		home, _ := os.UserHomeDir()
		dbPath = home + "/.config/helix-dev-tools/sprintboard.db"
	}
	return sql.Open("sqlite", dbPath)
}

// SetDB injects a pre-opened *sql.DB for testing.
func (f *SprintBoardFetcher) SetDB(db *sql.DB) { f.db = db }

func (f *SprintBoardFetcher) Fetch(ctx context.Context) (Status, error) {
	db, err := f.openDB()
	if err != nil {
		return Status{Level: "RED", Message: fmt.Sprintf("db open: %v", err)}, err
	}
	if f.db == nil {
		defer db.Close()
	}

	agents, err := f.fetchAgents(ctx, db)
	if err != nil {
		return Status{Level: "RED", Message: fmt.Sprintf("agents query: %v", err)}, err
	}
	tickets, err := f.fetchTickets(ctx, db)
	if err != nil {
		return Status{Level: "RED", Message: fmt.Sprintf("tickets query: %v", err)}, err
	}
	sprints, err := f.fetchSprints(ctx, db)
	if err != nil {
		return Status{Level: "RED", Message: fmt.Sprintf("sprints query: %v", err)}, err
	}

	data := map[string]interface{}{
		"agents":  agents,
		"tickets": tickets,
		"sprints": sprints,
	}
	return Status{Level: "GREEN", Message: fmt.Sprintf("%d agents, %d tickets, %d sprints", len(agents), len(tickets), len(sprints)), Data: data}, nil
}

func (f *SprintBoardFetcher) fetchAgents(ctx context.Context, db *sql.DB) ([]SprintBoardAgent, error) {
	rows, err := db.QueryContext(ctx, "SELECT id, surface, COALESCE(current_ticket_id,''), last_seen FROM agents ORDER BY last_seen DESC LIMIT 20")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var agents []SprintBoardAgent
	for rows.Next() {
		var a SprintBoardAgent
		if err := rows.Scan(&a.ID, &a.Surface, &a.CurrentTicketID, &a.LastSeen); err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

func (f *SprintBoardFetcher) fetchTickets(ctx context.Context, db *sql.DB) ([]SprintBoardTicket, error) {
	rows, err := db.QueryContext(ctx, "SELECT id, title, status, COALESCE(claimed_by,''), COALESCE(sprint_id,'') FROM tickets ORDER BY id DESC LIMIT 50")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tickets []SprintBoardTicket
	for rows.Next() {
		var t SprintBoardTicket
		if err := rows.Scan(&t.ID, &t.Title, &t.Status, &t.ClaimedBy, &t.SprintID); err != nil {
			return nil, err
		}
		tickets = append(tickets, t)
	}
	return tickets, rows.Err()
}

func (f *SprintBoardFetcher) fetchSprints(ctx context.Context, db *sql.DB) ([]SprintBoardSprint, error) {
	rows, err := db.QueryContext(ctx, "SELECT id, name, status FROM sprints ORDER BY created_at DESC LIMIT 10")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sprints []SprintBoardSprint
	for rows.Next() {
		var s SprintBoardSprint
		if err := rows.Scan(&s.ID, &s.Name, &s.Status); err != nil {
			return nil, err
		}
		sprints = append(sprints, s)
	}
	return sprints, rows.Err()
}

// --- Engram ---

// EngramFetcher checks the Engram healthz endpoint.
type EngramFetcher struct {
	BaseURL string // e.g. "http://localhost:8281"
	Client  *http.Client
}

func (f *EngramFetcher) Name() string { return "engram" }

func (f *EngramFetcher) Fetch(ctx context.Context) (Status, error) {
	client := f.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	url := f.BaseURL + "/healthz"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Status{Level: "RED", Message: fmt.Sprintf("request build: %v", err)}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return Status{Level: "RED", Message: fmt.Sprintf("engram unreachable: %v", err)}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusOK {
		return Status{Level: "GREEN", Message: "healthy", Data: string(body)}, nil
	}
	return Status{Level: "RED", Message: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))}, nil
}

// --- Agentrace ---

// AgentraceEvent is one parsed line from the agentrace NDJSON log.
type AgentraceEvent struct {
	Timestamp string `json:"ts"`
	Event     string `json:"event"`
	Query     string `json:"query,omitempty"`
	Pattern   string `json:"pattern,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

// AgentraceFetcher reads the tail of the agentrace NDJSON log.
type AgentraceFetcher struct {
	LogPath  string // defaults to ~/logs/runx/agentrace-mcp.ndjson
	TailSize int    // number of lines from the end to read (default 50)
}

func (f *AgentraceFetcher) Name() string { return "agentrace" }

func (f *AgentraceFetcher) Fetch(ctx context.Context) (Status, error) {
	logPath := f.LogPath
	if logPath == "" {
		home, _ := os.UserHomeDir()
		logPath = home + "/logs/runx/agentrace-mcp.ndjson"
	}
	tailSize := f.TailSize
	if tailSize <= 0 {
		tailSize = 50
	}

	events, err := parseTailNDJSON(logPath, tailSize)
	if err != nil {
		return Status{Level: "YELLOW", Message: fmt.Sprintf("read log: %v", err)}, err
	}
	if len(events) == 0 {
		return Status{Level: "YELLOW", Message: "no agentrace events found"}, nil
	}

	sembleCount, grepCount := 0, 0
	for _, e := range events {
		switch e.Event {
		case "semble_search":
			sembleCount++
		case "grep_fallback":
			grepCount++
		}
	}

	level := "GREEN"
	total := sembleCount + grepCount
	if total > 0 {
		semblePct := float64(sembleCount) / float64(total) * 100
		if semblePct < 50 {
			level = "RED"
		} else if semblePct < 80 {
			level = "YELLOW"
		}
	}

	data := map[string]interface{}{
		"events":       events,
		"semble_count": sembleCount,
		"grep_count":   grepCount,
		"total":        len(events),
	}
	return Status{Level: level, Message: fmt.Sprintf("%d events (%d semble, %d grep)", len(events), sembleCount, grepCount), Data: data}, nil
}

// parseTailNDJSON reads the last n lines from an NDJSON file and parses them.
func parseTailNDJSON(path string, n int) ([]AgentraceEvent, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var allLines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	start := 0
	if len(allLines) > n {
		start = len(allLines) - n
	}
	tail := allLines[start:]

	events := make([]AgentraceEvent, 0, len(tail))
	for _, line := range tail {
		if line == "" {
			continue
		}
		var e AgentraceEvent
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}
		events = append(events, e)
	}
	return events, nil
}

// ParseNDJSONReader parses NDJSON events from an io.Reader. Exported for testing.
func ParseNDJSONReader(r io.Reader, limit int) ([]AgentraceEvent, error) {
	scanner := bufio.NewScanner(r)
	var events []AgentraceEvent
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var e AgentraceEvent
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}
		events = append(events, e)
		if limit > 0 && len(events) >= limit {
			break
		}
	}
	return events, scanner.Err()
}
