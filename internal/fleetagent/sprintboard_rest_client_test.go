package fleetagent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRESTSprintBoardClient_ListReady(t *testing.T) {
	tickets := []Ticket{{ID: "t1", Title: "Build widget", Status: "backlog"}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/tickets/ready" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("sprint_id") != "v18400" {
			t.Errorf("unexpected sprint_id: %s", r.URL.Query().Get("sprint_id"))
		}
		json.NewEncoder(w).Encode(tickets)
	}))
	defer srv.Close()

	client := NewRESTSprintBoardClient(srv.URL, "v18400")
	result, err := client.ListReady(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 || result[0].ID != "t1" {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestRESTSprintBoardClient_ListReady_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("[]"))
	}))
	defer srv.Close()

	client := NewRESTSprintBoardClient(srv.URL, "v18400")
	result, err := client.ListReady(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty, got %d tickets", len(result))
	}
}

func TestRESTSprintBoardClient_Claim_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/tickets/t1/claim" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["agent_id"] != "fleet-1" {
			t.Errorf("unexpected agent_id: %s", body["agent_id"])
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}))
	defer srv.Close()

	client := NewRESTSprintBoardClient(srv.URL, "v18400")
	result, err := client.Claim(context.Background(), "t1", "fleet-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected successful claim")
	}
}

func TestRESTSprintBoardClient_Claim_Conflict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{
			"error":      "already claimed",
			"claimed_by": "other-agent",
		})
	}))
	defer srv.Close()

	client := NewRESTSprintBoardClient(srv.URL, "v18400")
	result, err := client.Claim(context.Background(), "t1", "fleet-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected conflict, got success")
	}
	if result.ConflictBy != "other-agent" {
		t.Errorf("unexpected conflict_by: %s", result.ConflictBy)
	}
}

func TestRESTSprintBoardClient_Complete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/tickets/t1/complete" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["evidence"] != "all tests pass" {
			t.Errorf("unexpected evidence: %s", body["evidence"])
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewRESTSprintBoardClient(srv.URL, "v18400")
	err := client.Complete(context.Background(), "t1", "fleet-1", "all tests pass")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRESTSprintBoardClient_Block(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/tickets/t1/block" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewRESTSprintBoardClient(srv.URL, "v18400")
	err := client.Block(context.Background(), "t1", "fleet-1", "build failed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRESTSprintBoardClient_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	client := NewRESTSprintBoardClient(srv.URL, "v18400")
	_, err := client.ListReady(context.Background(), nil)
	if err == nil {
		t.Error("expected error for 500 response")
	}
}
