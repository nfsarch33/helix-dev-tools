package dashboard

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// --- GitLab fetcher tests ---

func TestGitLabFetcher_Success(t *testing.T) {
	pipelines := []GitLabPipeline{
		{ID: 1, Status: "success", Ref: "main", CreatedAt: "2026-05-25T00:00:00Z"},
		{ID: 2, Status: "success", Ref: "feat/x", CreatedAt: "2026-05-24T00:00:00Z"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/api/v4/projects/")
		assert.Equal(t, "test-token", r.Header.Get("PRIVATE-TOKEN"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(pipelines)
	}))
	defer srv.Close()

	f := &GitLabFetcher{
		BaseURL:    srv.URL,
		ProjectIDs: []int{42},
		Token:      "test-token",
		Client:     srv.Client(),
	}
	status, err := f.Fetch(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "GREEN", status.Level)
	assert.Equal(t, "pipelines fetched", status.Message)

	data := status.Data.(map[int][]GitLabPipeline)
	assert.Len(t, data[42], 2)
}

func TestGitLabFetcher_FailedPipeline(t *testing.T) {
	pipelines := []GitLabPipeline{
		{ID: 1, Status: "failed", Ref: "main"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(pipelines)
	}))
	defer srv.Close()

	f := &GitLabFetcher{
		BaseURL:    srv.URL,
		ProjectIDs: []int{1},
		Client:     srv.Client(),
	}
	status, err := f.Fetch(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "RED", status.Level)
}

func TestGitLabFetcher_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	f := &GitLabFetcher{
		BaseURL:    srv.URL,
		ProjectIDs: []int{1},
		Client:     srv.Client(),
	}
	status, err := f.Fetch(ctx)
	require.Error(t, err)
	assert.Equal(t, "RED", status.Level)
	assert.Contains(t, status.Message, "unreachable")
}

func TestGitLabFetcher_Name(t *testing.T) {
	f := &GitLabFetcher{}
	assert.Equal(t, "gitlab", f.Name())
}

// --- ArgoCD fetcher tests ---

func TestArgoCDFetcher_Success(t *testing.T) {
	argoResp := `{
		"items": [
			{"metadata":{"name":"app1"},"status":{"sync":{"status":"Synced"},"health":{"status":"Healthy"}}},
			{"metadata":{"name":"app2"},"status":{"sync":{"status":"Synced"},"health":{"status":"Healthy"}}}
		]
	}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/applications", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(argoResp))
	}))
	defer srv.Close()

	f := &ArgoCDFetcher{BaseURL: srv.URL, Client: srv.Client()}
	status, err := f.Fetch(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "GREEN", status.Level)
	apps := status.Data.([]ArgoCDApp)
	assert.Len(t, apps, 2)
}

func TestArgoCDFetcher_Degraded(t *testing.T) {
	argoResp := `{
		"items": [
			{"metadata":{"name":"app1"},"status":{"sync":{"status":"Synced"},"health":{"status":"Degraded"}}}
		]
	}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(argoResp))
	}))
	defer srv.Close()

	f := &ArgoCDFetcher{BaseURL: srv.URL, Client: srv.Client()}
	status, err := f.Fetch(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "RED", status.Level)
}

// --- SprintBoard fetcher tests ---

func TestSprintBoardFetcher_ReadTickets(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE agents (id TEXT PRIMARY KEY, surface TEXT, current_ticket_id TEXT, last_seen TEXT);
		CREATE TABLE tickets (id TEXT PRIMARY KEY, title TEXT, status TEXT, claimed_by TEXT, sprint_id TEXT);
		CREATE TABLE sprints (id TEXT PRIMARY KEY, name TEXT, status TEXT, created_at TEXT);

		INSERT INTO agents VALUES ('a1', 'cursor-parent', 'v100-1', '2026-05-25T00:00:00+10:00');
		INSERT INTO tickets VALUES ('v100-1', 'Build dashboard', 'in_progress', 'a1', 'v100');
		INSERT INTO tickets VALUES ('v100-2', 'Write tests', 'backlog', '', 'v100');
		INSERT INTO sprints VALUES ('v100', 'Dashboard Sprint', 'active', '2026-05-25T00:00:00+10:00');
	`)
	require.NoError(t, err)

	f := &SprintBoardFetcher{}
	f.SetDB(db)
	status, err := f.Fetch(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "GREEN", status.Level)

	data := status.Data.(map[string]interface{})
	agents := data["agents"].([]SprintBoardAgent)
	tickets := data["tickets"].([]SprintBoardTicket)
	sprints := data["sprints"].([]SprintBoardSprint)
	assert.Len(t, agents, 1)
	assert.Len(t, tickets, 2)
	assert.Len(t, sprints, 1)
	assert.Equal(t, "cursor-parent", agents[0].Surface)
	assert.Equal(t, "Build dashboard", tickets[0].Title)
}

func TestSprintBoardFetcher_Name(t *testing.T) {
	f := &SprintBoardFetcher{}
	assert.Equal(t, "sprintboard", f.Name())
}

// --- Engram fetcher tests ---

func TestEngramFetcher_Healthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/healthz", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	f := &EngramFetcher{BaseURL: srv.URL, Client: srv.Client()}
	status, err := f.Fetch(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "GREEN", status.Level)
	assert.Equal(t, "healthy", status.Message)
}

func TestEngramFetcher_Unhealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("down"))
	}))
	defer srv.Close()

	f := &EngramFetcher{BaseURL: srv.URL, Client: srv.Client()}
	status, err := f.Fetch(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "RED", status.Level)
}

func TestEngramFetcher_Unreachable(t *testing.T) {
	f := &EngramFetcher{BaseURL: "http://127.0.0.1:1", Client: &http.Client{Timeout: 100 * time.Millisecond}}
	status, err := f.Fetch(context.Background())
	require.Error(t, err)
	assert.Equal(t, "RED", status.Level)
	assert.Contains(t, status.Message, "unreachable")
}

// --- Agentrace fetcher tests ---

func TestAgentraceFetcher_ParseNDJSON(t *testing.T) {
	ndjson := strings.Join([]string{
		`{"ts":"2026-05-25T00:01:00+10:00","event":"semble_search","query":"auth flow"}`,
		`{"ts":"2026-05-25T00:02:00+10:00","event":"grep_fallback","pattern":"ErrNotFound","reason":"exact-literal"}`,
		`{"ts":"2026-05-25T00:03:00+10:00","event":"semble_search","query":"config loading"}`,
	}, "\n")

	events, err := ParseNDJSONReader(strings.NewReader(ndjson), 0)
	require.NoError(t, err)
	assert.Len(t, events, 3)
	assert.Equal(t, "semble_search", events[0].Event)
	assert.Equal(t, "auth flow", events[0].Query)
	assert.Equal(t, "grep_fallback", events[1].Event)
	assert.Equal(t, "ErrNotFound", events[1].Pattern)
}

func TestAgentraceFetcher_ParseNDJSON_WithLimit(t *testing.T) {
	ndjson := strings.Join([]string{
		`{"ts":"t1","event":"semble_search","query":"a"}`,
		`{"ts":"t2","event":"semble_search","query":"b"}`,
		`{"ts":"t3","event":"semble_search","query":"c"}`,
	}, "\n")

	events, err := ParseNDJSONReader(strings.NewReader(ndjson), 2)
	require.NoError(t, err)
	assert.Len(t, events, 2)
}

func TestAgentraceFetcher_FileRead(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "trace.ndjson")
	lines := []string{
		`{"ts":"t1","event":"semble_search","query":"foo"}`,
		`{"ts":"t2","event":"grep_fallback","pattern":"bar","reason":"exact-literal"}`,
		`{"ts":"t3","event":"semble_search","query":"baz"}`,
		`{"ts":"t4","event":"semble_search","query":"qux"}`,
		`{"ts":"t5","event":"semble_search","query":"quux"}`,
	}
	require.NoError(t, os.WriteFile(logPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644))

	f := &AgentraceFetcher{LogPath: logPath, TailSize: 5}
	status, err := f.Fetch(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "GREEN", status.Level)

	data := status.Data.(map[string]interface{})
	assert.Equal(t, 4, data["semble_count"])
	assert.Equal(t, 1, data["grep_count"])
}

func TestAgentraceFetcher_MissingFile(t *testing.T) {
	f := &AgentraceFetcher{LogPath: "/nonexistent/trace.ndjson"}
	status, err := f.Fetch(context.Background())
	require.Error(t, err)
	assert.Equal(t, "YELLOW", status.Level)
}

// --- Fetcher interface compliance ---

func TestFetcherInterfaceCompliance(t *testing.T) {
	fetchers := []Fetcher{
		&GitLabFetcher{},
		&ArgoCDFetcher{},
		&SprintBoardFetcher{},
		&EngramFetcher{},
		&AgentraceFetcher{},
	}
	names := map[string]bool{}
	for _, f := range fetchers {
		name := f.Name()
		assert.NotEmpty(t, name, "fetcher Name() must not be empty")
		assert.False(t, names[name], "duplicate fetcher name: %s", name)
		names[name] = true
	}
}
