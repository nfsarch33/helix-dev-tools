package cicd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebhookHandler_PipelineFailure(t *testing.T) {
	var notified bool
	engram := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		notified = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"mem-1"}`))
	}))
	defer engram.Close()

	notifier := NewNotifier(engram.URL, "user", "app", "")
	handler := &WebhookHandler{Notifier: notifier}

	payload := gitlabWebhookPayload{ObjectKind: "pipeline"}
	payload.ObjectAttr.ID = 42
	payload.ObjectAttr.Status = StatusFailed
	payload.ObjectAttr.Ref = "main"
	payload.Project.Name = "engram"
	payload.Project.WebURL = "https://gitlab.example.com/engram"
	body, _ := json.Marshal(payload)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(string(body)))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if !notified {
		t.Error("expected notification to Engram")
	}
}

func TestWebhookHandler_SecretValidation(t *testing.T) {
	notifier := NewNotifier("http://unused", "u", "a", "")
	handler := &WebhookHandler{Secret: "my-secret", Notifier: notifier}

	t.Run("rejects missing token", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader("{}"))
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("accepts valid token", func(t *testing.T) {
		payload := `{"object_kind":"push"}`
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(payload))
		req.Header.Set("X-Gitlab-Token", "my-secret")
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})
}

func TestWebhookHandler_IgnoresNonPipeline(t *testing.T) {
	notifier := NewNotifier("http://unused", "u", "a", "")
	handler := &WebhookHandler{Notifier: notifier}

	payload := `{"object_kind":"push"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(payload))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "ignored event: push") {
		t.Errorf("expected ignored message, got: %s", rec.Body.String())
	}
}

func TestWebhookHandler_RejectsGET(t *testing.T) {
	handler := &WebhookHandler{Notifier: &Notifier{}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}
