package fleetagent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPLLMClient_Complete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		resp := chatResponse{
			Choices: []chatChoice{
				{Message: chatMessage{Role: "assistant", Content: "Task completed successfully."}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewHTTPLLMClient(server.URL, "qwen-3.5-4b", "test-key")
	result, err := client.Complete(context.Background(), "system", "do something")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Task completed successfully." {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestHTTPLLMClient_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	client := NewHTTPLLMClient(server.URL, "model", "")
	_, err := client.Complete(context.Background(), "sys", "user")
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestHTTPLLMClient_EmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(chatResponse{Choices: nil})
	}))
	defer server.Close()

	client := NewHTTPLLMClient(server.URL, "model", "")
	_, err := client.Complete(context.Background(), "sys", "user")
	if err == nil {
		t.Error("expected error for empty choices")
	}
}

func TestHTTPSprintBoardClient_ListReady(t *testing.T) {
	tickets := []Ticket{
		{ID: "t1", Title: "Test ticket", Status: "ready"},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := mcpResponse{JSONRPC: "2.0", ID: 1}
		b, _ := json.Marshal(tickets)
		resp.Result = b
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewHTTPSprintBoardClient(server.URL)
	result, err := client.ListReady(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 ticket, got %d", len(result))
	}
	if result[0].ID != "t1" {
		t.Errorf("expected ticket ID t1, got %s", result[0].ID)
	}
}

func TestHTTPSprintBoardClient_Claim(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cr := ClaimResult{Success: true, TicketID: "t1"}
		resp := mcpResponse{JSONRPC: "2.0", ID: 1}
		b, _ := json.Marshal(cr)
		resp.Result = b
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewHTTPSprintBoardClient(server.URL)
	result, err := client.Claim(context.Background(), "t1", "agent-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected successful claim")
	}
}

func TestHTTPSprintBoardClient_MCPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := mcpResponse{
			JSONRPC: "2.0",
			ID:      1,
			Error:   &mcpError{Code: -32603, Message: "ticket not found"},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewHTTPSprintBoardClient(server.URL)
	_, err := client.ListReady(context.Background(), nil)
	if err == nil {
		t.Error("expected error for MCP error response")
	}
}

func TestEngramReporter_Report(t *testing.T) {
	var receivedBody engramAddRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/memories/" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"mem-1"}`))
	}))
	defer server.Close()

	reporter := NewEngramReporter(server.URL, "nfsarch33", "fleet-agent", "test-key")
	result := ExecutionResult{
		TicketID:  "t1",
		Success:   true,
		Output:    "done",
		Duration:  5 * time.Second,
		Timestamp: time.Now(),
	}
	err := reporter.Report(context.Background(), result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedBody.UserID != "nfsarch33" {
		t.Errorf("expected user_id nfsarch33, got %s", receivedBody.UserID)
	}
	if receivedBody.AppID != "fleet-agent" {
		t.Errorf("expected app_id fleet-agent, got %s", receivedBody.AppID)
	}
}

func TestEngramReporter_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("unavailable"))
	}))
	defer server.Close()

	reporter := NewEngramReporter(server.URL, "user", "app", "")
	result := ExecutionResult{TicketID: "t1", Success: false, Error: "failed"}
	err := reporter.Report(context.Background(), result)
	if err == nil {
		t.Error("expected error for 503 response")
	}
}
