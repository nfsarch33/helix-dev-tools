package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubFetcher is a test double that returns a fixed Status.
type stubFetcher struct {
	name   string
	status Status
	err    error
}

func (f *stubFetcher) Name() string                             { return f.name }
func (f *stubFetcher) Fetch(_ context.Context) (Status, error) { return f.status, f.err }

func newTestServer(t *testing.T, fetchers ...Fetcher) *Server {
	t.Helper()
	srv, err := New(fetchers, "", ":0")
	require.NoError(t, err)
	return srv
}

func TestDashboardServer_Routes(t *testing.T) {
	stub := &stubFetcher{name: "test", status: Status{Level: "GREEN", Message: "ok"}}
	srv := newTestServer(t, stub)
	handler := srv.Handler()

	routes := []struct {
		path string
		code int
	}{
		{"/", http.StatusOK},
		{"/ci", http.StatusOK},
		{"/agents", http.StatusOK},
		{"/sprints", http.StatusOK},
		{"/fleet", http.StatusOK},
		{"/roadmap", http.StatusOK},
		{"/api/health", http.StatusOK},
	}
	for _, tc := range routes {
		t.Run(tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			handler.ServeHTTP(rec, req)
			assert.Equal(t, tc.code, rec.Code, "route %s", tc.path)
		})
	}
}

func TestDashboardServer_OverviewContainsBadge(t *testing.T) {
	stub := &stubFetcher{name: "engram", status: Status{Level: "GREEN", Message: "healthy"}}
	srv := newTestServer(t, stub)
	handler := srv.Handler()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	body := rec.Body.String()
	assert.Contains(t, body, "badge-GREEN")
	assert.Contains(t, body, "engram")
	assert.Contains(t, body, "healthy")
}

func TestDashboardServer_OverviewRedBadge(t *testing.T) {
	stub := &stubFetcher{name: "gitlab", status: Status{Level: "RED", Message: "down"}}
	srv := newTestServer(t, stub)
	handler := srv.Handler()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Contains(t, rec.Body.String(), "badge-RED")
}

func TestDashboardServer_APIHealth(t *testing.T) {
	stub := &stubFetcher{name: "test", status: Status{Level: "YELLOW", Message: "pending"}}
	srv := newTestServer(t, stub)
	handler := srv.Handler()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
	assert.Contains(t, rec.Body.String(), "YELLOW")
}

func TestDashboardServer_RoadmapWithManifest(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "roadmap.yaml")
	require.NoError(t, os.WriteFile(manifest, []byte(`
components:
  - name: Dashboard
    status: MVP
    pct: 75
  - name: SprintBoard
    status: Done
    pct: 100
`), 0o644))

	srv, err := New(nil, manifest, ":0")
	require.NoError(t, err)
	handler := srv.Handler()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/roadmap", nil))
	body := rec.Body.String()
	assert.Contains(t, body, "Dashboard")
	assert.Contains(t, body, "75%")
	assert.Contains(t, body, "badge-YELLOW")
	assert.Contains(t, body, "badge-GREEN")
}

func TestDashboardServer_404ForUnknownPath(t *testing.T) {
	srv := newTestServer(t)
	handler := srv.Handler()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nonexistent", nil))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestDashboardServer_MultipleFetchers(t *testing.T) {
	fetchers := []Fetcher{
		&stubFetcher{name: "gitlab", status: Status{Level: "GREEN", Message: "ok"}},
		&stubFetcher{name: "engram", status: Status{Level: "RED", Message: "down"}},
		&stubFetcher{name: "sprintboard", status: Status{Level: "YELLOW", Message: "stale"}},
	}
	srv := newTestServer(t, fetchers...)
	handler := srv.Handler()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	body := rec.Body.String()
	assert.Contains(t, body, "gitlab")
	assert.Contains(t, body, "engram")
	assert.Contains(t, body, "sprintboard")
	assert.Contains(t, body, "badge-GREEN")
	assert.Contains(t, body, "badge-RED")
	assert.Contains(t, body, "badge-YELLOW")
}
