package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFleetHealthEndpoint_ReturnsJSON(t *testing.T) {
	srv := newTestServer(t)
	handler := srv.Handler()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/fleet/health", nil))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")

	var resp FleetHealthResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Timestamp)
	assert.NotNil(t, resp.Probes)
}

func TestFleetHealthEndpoint_MethodNotAllowed(t *testing.T) {
	srv := newTestServer(t)
	handler := srv.Handler()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/fleet/health", nil))
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestFleetHealthEndpoint_CachesResults(t *testing.T) {
	srv := newTestServer(t)
	handler := srv.Handler()

	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/api/fleet/health", nil))
	require.Equal(t, http.StatusOK, rec1.Code)

	var resp1 FleetHealthResponse
	require.NoError(t, json.Unmarshal(rec1.Body.Bytes(), &resp1))

	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/api/fleet/health", nil))
	require.Equal(t, http.StatusOK, rec2.Code)

	var resp2 FleetHealthResponse
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &resp2))

	assert.Equal(t, resp1.Timestamp, resp2.Timestamp, "second call should return cached result")
}

func TestBuildFleetHealthResponse_EmptyNodes(t *testing.T) {
	resp := buildFleetHealthResponse(nil, time.Millisecond)
	assert.NotEmpty(t, resp.Timestamp)
	assert.Empty(t, resp.Probes)
	assert.Equal(t, FleetHealthSummary{}, resp.Summary)
}

func TestBuildFleetHealthResponse_WithNodes(t *testing.T) {
	nodes := []fleetNode{
		{
			Name:    "macbook",
			Level:   "GREEN",
			FreePct: 45,
			Services: []fleetService{
				{Name: "engram", Status: "up"},
				{Name: "vllm", Status: "down"},
			},
		},
		{
			Name:    "gpu-host",
			Level:   "YELLOW",
			FreePct: 10,
			Services: []fleetService{
				{Name: "k3s", Status: "up"},
			},
		},
	}
	resp := buildFleetHealthResponse(nodes, 5*time.Millisecond)

	assert.Len(t, resp.Probes, 5)

	assert.Equal(t, "macbook", resp.Probes[0].Name)
	assert.Equal(t, "green", resp.Probes[0].Status)

	assert.Equal(t, "macbook/engram", resp.Probes[1].Name)
	assert.Equal(t, "green", resp.Probes[1].Status)

	assert.Equal(t, "macbook/vllm", resp.Probes[2].Name)
	assert.Equal(t, "red", resp.Probes[2].Status)

	assert.Equal(t, "gpu-host", resp.Probes[3].Name)
	assert.Equal(t, "yellow", resp.Probes[3].Status)

	assert.Equal(t, "gpu-host/k3s", resp.Probes[4].Name)
	assert.Equal(t, "green", resp.Probes[4].Status)

	assert.Equal(t, 3, resp.Summary.Green)
	assert.Equal(t, 1, resp.Summary.Yellow)
	assert.Equal(t, 1, resp.Summary.Red)
}

func TestBuildFleetHealthResponse_RedNode(t *testing.T) {
	nodes := []fleetNode{
		{Name: "critical-host", Level: "RED", FreePct: 2},
	}
	resp := buildFleetHealthResponse(nodes, time.Millisecond)
	require.Len(t, resp.Probes, 1)
	assert.Equal(t, "red", resp.Probes[0].Status)
	assert.Equal(t, 0, resp.Summary.Green)
	assert.Equal(t, 1, resp.Summary.Red)
}

func TestStatusFromLevel(t *testing.T) {
	tests := []struct {
		level    string
		expected string
	}{
		{"GREEN", "green"},
		{"YELLOW", "yellow"},
		{"RED", "red"},
		{"UNKNOWN", "red"},
		{"", "red"},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.expected, statusFromLevel(tc.level), "level=%q", tc.level)
	}
}

func TestStatusFromServiceStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"up", "green"},
		{"down", "red"},
		{"unknown", "yellow"},
		{"", "yellow"},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.expected, statusFromServiceStatus(tc.input), "input=%q", tc.input)
	}
}

func TestFleetHealthCache_SetGet(t *testing.T) {
	c := &fleetHealthCache{}
	assert.Nil(t, c.get())

	resp := &FleetHealthResponse{Timestamp: "2026-05-26T14:52+10:00"}
	c.set(resp, time.Minute)
	assert.Equal(t, resp, c.get())
}

func TestFleetHealthCache_Expiry(t *testing.T) {
	c := &fleetHealthCache{}
	resp := &FleetHealthResponse{Timestamp: "2026-05-26T14:52+10:00"}
	c.set(resp, time.Nanosecond)
	time.Sleep(time.Millisecond)
	assert.Nil(t, c.get(), "expired cache should return nil")
}
