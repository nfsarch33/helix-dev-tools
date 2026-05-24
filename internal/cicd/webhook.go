package cicd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

// WebhookHandler receives GitLab pipeline webhooks and notifies on failures.
type WebhookHandler struct {
	Secret   string
	Notifier *Notifier
}

type gitlabWebhookPayload struct {
	ObjectKind string `json:"object_kind"`
	ObjectAttr struct {
		ID     int    `json:"id"`
		Status string `json:"status"`
		Ref    string `json:"ref"`
		SHA    string `json:"sha"`
	} `json:"object_attributes"`
	Project struct {
		Name   string `json:"name"`
		WebURL string `json:"web_url"`
	} `json:"project"`
}

// ServeHTTP handles incoming GitLab webhook events.
func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.Secret != "" {
		token := r.Header.Get("X-Gitlab-Token")
		if token != h.Secret {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}

	var payload gitlabWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if payload.ObjectKind != "pipeline" {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "ignored event: %s", payload.ObjectKind)
		return
	}

	pipeline := Pipeline{
		ID:     payload.ObjectAttr.ID,
		Status: payload.ObjectAttr.Status,
		Ref:    payload.ObjectAttr.Ref,
		SHA:    payload.ObjectAttr.SHA,
		WebURL: fmt.Sprintf("%s/-/pipelines/%d", payload.Project.WebURL, payload.ObjectAttr.ID),
	}

	ctx := context.Background()
	switch payload.ObjectAttr.Status {
	case StatusFailed:
		if err := h.Notifier.NotifyFailure(ctx, payload.Project.Name, pipeline); err != nil {
			log.Printf("webhook: notify failure: %v", err)
		}
	case StatusSuccess:
		if err := h.Notifier.NotifySuccess(ctx, payload.Project.Name, pipeline); err != nil {
			log.Printf("webhook: notify success: %v", err)
		}
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "processed pipeline %d status=%s", pipeline.ID, pipeline.Status)
}
