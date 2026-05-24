package helixone2e

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHelixonClient_HealthCheck_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	c := NewHelixonClient(server.URL)
	err := c.HealthCheck(context.Background())
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestHelixonClient_HealthCheck_Down(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	c := NewHelixonClient(server.URL)
	err := c.HealthCheck(context.Background())
	if err == nil {
		t.Error("expected error for unhealthy service")
	}
}

func TestHelixonClient_SubmitTask(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/task" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var req TaskRequest
		json.NewDecoder(r.Body).Decode(&req)
		resp := TaskResponse{TaskID: "task-1", Status: "completed", Response: "Done: " + req.Prompt}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := NewHelixonClient(server.URL)
	resp, err := c.SubmitTask(context.Background(), TaskRequest{Prompt: "test task"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "completed" {
		t.Errorf("expected completed, got %s", resp.Status)
	}
	if resp.TaskID != "task-1" {
		t.Errorf("expected task-1, got %s", resp.TaskID)
	}
}

func TestHelixonClient_SubmitTask_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	c := NewHelixonClient(server.URL)
	_, err := c.SubmitTask(context.Background(), TaskRequest{Prompt: "fail"})
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestEngramVerifier_SearchMemory_Found(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]string{{"memory": "found it"}})
	}))
	defer server.Close()

	v := NewEngramVerifier(server.URL, "key")
	found, err := v.SearchMemory(context.Background(), "test", "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Error("expected found=true")
	}
}

func TestEngramVerifier_SearchMemory_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]string{})
	}))
	defer server.Close()

	v := NewEngramVerifier(server.URL, "")
	found, err := v.SearchMemory(context.Background(), "xyz", "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Error("expected found=false for empty results")
	}
}
