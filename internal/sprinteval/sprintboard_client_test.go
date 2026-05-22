package sprinteval

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestFetchSprintMetrics_DerivesCompletionFromHistogram exercises the
// real sprintboard envelope shape: {sprint, tickets_by_status, total_tickets}.
// The function must derive completion_rate, completed counts, in_progress
// and blocked counts from the histogram.
func TestFetchSprintMetrics_DerivesCompletionFromHistogram(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/sprints/v8000" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"sprint": map[string]any{
				"id":     "v8000",
				"name":   "v8000 overnight",
				"status": "active",
			},
			"tickets_by_status": map[string]int{
				"done":        12,
				"in_progress": 3,
				"blocked":     1,
				"backlog":     4,
			},
			"total_tickets": 20,
		})
	}))
	defer srv.Close()

	data, err := FetchSprintMetrics(srv.URL, "v8000")
	if err != nil {
		t.Fatalf("FetchSprintMetrics: %v", err)
	}
	if data.SprintID != "v8000" {
		t.Errorf("SprintID = %q, want v8000", data.SprintID)
	}
	if data.SprintName != "v8000 overnight" {
		t.Errorf("SprintName = %q, want v8000 overnight", data.SprintName)
	}
	if data.TotalTickets != 20 {
		t.Errorf("TotalTickets = %d, want 20", data.TotalTickets)
	}
	if data.CompletedTickets != 12 {
		t.Errorf("CompletedTickets = %d, want 12", data.CompletedTickets)
	}
	if data.InProgress != 3 {
		t.Errorf("InProgress = %d, want 3", data.InProgress)
	}
	if data.Blocked != 1 {
		t.Errorf("Blocked = %d, want 1", data.Blocked)
	}
	if data.CompletionRate < 0.59 || data.CompletionRate > 0.61 {
		t.Errorf("CompletionRate = %f, want ~0.6", data.CompletionRate)
	}
}

// TestFetchSprintMetrics_NotFoundIsTyped confirms a 404 from sprintboard
// returns a not-found error so callers can distinguish from network
// failures.
func TestFetchSprintMetrics_NotFoundIsTyped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.NotFound(w, nil)
	}))
	defer srv.Close()

	_, err := FetchSprintMetrics(srv.URL, "missing")
	if err == nil {
		t.Fatal("expected not-found error")
	}
	if !contains(err.Error(), "not found") {
		t.Errorf("err = %q, want it to mention 'not found'", err.Error())
	}
}

// TestFetchSprintTickets_GreenPath confirms the /sprints/{id}/tickets
// endpoint returns ticket snapshots that drop straight into ComputeMetrics.
func TestFetchSprintTickets_GreenPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/sprints/v8000/tickets" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"sprint_id": "v8000",
			"tickets": []map[string]any{
				{"id": "T-8000-B1", "title": "helixon", "status": "done", "priority": 1},
				{"id": "T-8000-B9", "title": "ndjson", "status": "in_progress", "priority": 1},
			},
		})
	}))
	defer srv.Close()

	tickets, err := FetchSprintTickets(srv.URL, "v8000")
	if err != nil {
		t.Fatalf("FetchSprintTickets: %v", err)
	}
	if len(tickets) != 2 {
		t.Fatalf("tickets = %d, want 2", len(tickets))
	}
	if tickets[0].ID != "T-8000-B1" || tickets[0].Status != "done" {
		t.Errorf("tickets[0] = %+v", tickets[0])
	}

	m := ComputeMetrics(tickets, nil, nil, nil)
	if m.TotalTickets != 2 || m.CompletedTickets != 1 {
		t.Errorf("metrics totals = %d/%d, want 2/1", m.CompletedTickets, m.TotalTickets)
	}
	if m.CompletionRate < 0.49 || m.CompletionRate > 0.51 {
		t.Errorf("CompletionRate = %f, want ~0.5", m.CompletionRate)
	}
}

// TestFetchSprintTickets_OldServerReturns404 documents the fallback
// path when the sprintboard binary is older than B19 (no /tickets route).
// The caller should fall back to FetchSprintMetrics + SnapshotsFromHistogram.
func TestFetchSprintTickets_OldServerReturns404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.NotFound(w, nil)
	}))
	defer srv.Close()

	_, err := FetchSprintTickets(srv.URL, "v8000")
	if err == nil {
		t.Fatal("expected error for old server")
	}
	if !contains(err.Error(), "tickets endpoint not found") {
		t.Errorf("err = %q, want it to mention older sprintboard", err.Error())
	}
}

// TestSnapshotsFromHistogram_PreservesCompletionRatio confirms the
// fallback synthesis preserves the completion-rate signal even with
// fabricated ticket IDs.
func TestSnapshotsFromHistogram_PreservesCompletionRatio(t *testing.T) {
	hist := map[string]int{
		"done":        7,
		"in_progress": 2,
		"backlog":     1,
	}

	snaps := SnapshotsFromHistogram(hist)
	if len(snaps) != 10 {
		t.Fatalf("snaps = %d, want 10", len(snaps))
	}

	m := ComputeMetrics(snaps, nil, nil, nil)
	if m.TotalTickets != 10 || m.CompletedTickets != 7 {
		t.Errorf("metrics totals = %d/%d, want 7/10", m.CompletedTickets, m.TotalTickets)
	}
	if m.CompletionRate < 0.69 || m.CompletionRate > 0.71 {
		t.Errorf("CompletionRate = %f, want ~0.7", m.CompletionRate)
	}
}

func TestSnapshotsFromHistogram_EmptyReturnsNil(t *testing.T) {
	if got := SnapshotsFromHistogram(nil); got != nil {
		t.Errorf("SnapshotsFromHistogram(nil) = %v, want nil", got)
	}
}
