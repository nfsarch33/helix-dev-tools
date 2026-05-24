package cicd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNotifier_NotifyFailure(t *testing.T) {
	var received map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/memories/" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"mem-1"}`))
	}))
	defer server.Close()

	n := NewNotifier(server.URL, "nfsarch33", "ci-notify", "test-key")
	pipeline := Pipeline{ID: 42, Status: StatusFailed, Ref: "main", WebURL: "https://example.com/42"}
	err := n.NotifyFailure(context.Background(), "my-project", pipeline)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if received["user_id"] != "nfsarch33" {
		t.Errorf("expected user_id nfsarch33, got %v", received["user_id"])
	}
}

func TestNotifier_NotifyFailure_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	n := NewNotifier(server.URL, "user", "app", "")
	err := n.NotifyFailure(context.Background(), "proj", Pipeline{ID: 1, Status: StatusFailed})
	if err == nil {
		t.Error("expected error for 503")
	}
}

func TestNotifier_NotifySuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"mem-2"}`))
	}))
	defer server.Close()

	n := NewNotifier(server.URL, "user", "app", "key")
	err := n.NotifySuccess(context.Background(), "proj", Pipeline{ID: 1, Ref: "main"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
