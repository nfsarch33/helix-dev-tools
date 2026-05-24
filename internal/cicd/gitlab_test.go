package cicd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitLabClient_GetProject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/projects/1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("PRIVATE-TOKEN") != "test-token" {
			t.Error("missing PRIVATE-TOKEN header")
		}
		json.NewEncoder(w).Encode(Project{ID: 1, Name: "my-project", PathNS: "group/my-project"})
	}))
	defer server.Close()

	client := NewGitLabClient(server.URL, "test-token")
	p, err := client.GetProject(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name != "my-project" {
		t.Errorf("expected name my-project, got %s", p.Name)
	}
}

func TestGitLabClient_TriggerPipeline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Pipeline{ID: 42, Status: StatusPending, Ref: "main"})
	}))
	defer server.Close()

	client := NewGitLabClient(server.URL, "token")
	p, err := client.TriggerPipeline(context.Background(), 1, "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.ID != 42 {
		t.Errorf("expected pipeline ID 42, got %d", p.ID)
	}
	if p.Status != StatusPending {
		t.Errorf("expected status pending, got %s", p.Status)
	}
}

func TestGitLabClient_GetPipelineStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(Pipeline{ID: 42, Status: StatusSuccess})
	}))
	defer server.Close()

	client := NewGitLabClient(server.URL, "token")
	p, err := client.GetPipelineStatus(context.Background(), 1, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Status != StatusSuccess {
		t.Errorf("expected success, got %s", p.Status)
	}
}

func TestGitLabClient_ListPipelines(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pipelines := []Pipeline{
			{ID: 1, Status: StatusSuccess, Ref: "main"},
			{ID: 2, Status: StatusFailed, Ref: "feat/x"},
		}
		json.NewEncoder(w).Encode(pipelines)
	}))
	defer server.Close()

	client := NewGitLabClient(server.URL, "token")
	pipelines, err := client.ListPipelines(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pipelines) != 2 {
		t.Errorf("expected 2 pipelines, got %d", len(pipelines))
	}
}

func TestGitLabClient_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"404 Project Not Found"}`))
	}))
	defer server.Close()

	client := NewGitLabClient(server.URL, "token")
	_, err := client.GetProject(context.Background(), 999)
	if err == nil {
		t.Error("expected error for 404")
	}
}
