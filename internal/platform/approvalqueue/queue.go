package approvalqueue

import (
	"fmt"
	"sync"
	"time"
)

type RequestStatus string

const (
	StatusPending  RequestStatus = "pending"
	StatusApproved RequestStatus = "approved"
	StatusDenied   RequestStatus = "denied"
)

type Request struct {
	ID          string        `json:"id"`
	Agent       string        `json:"agent"`
	Action      string        `json:"action"`
	Reason      string        `json:"reason,omitempty"`
	Status      RequestStatus `json:"status"`
	SubmittedAt time.Time     `json:"submitted_at"`
	ReviewedAt  time.Time     `json:"reviewed_at,omitempty"`
	ReviewedBy  string        `json:"reviewed_by,omitempty"`
	DenyReason  string        `json:"deny_reason,omitempty"`
}

type QueueStats struct {
	Total    int `json:"total"`
	Pending  int `json:"pending"`
	Approved int `json:"approved"`
	Denied   int `json:"denied"`
}

type Queue struct {
	mu       sync.RWMutex
	requests map[string]Request
}

func New() *Queue {
	return &Queue{
		requests: make(map[string]Request),
	}
}

func (q *Queue) Submit(req Request) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if _, exists := q.requests[req.ID]; exists {
		return fmt.Errorf("request %q already exists", req.ID)
	}

	if req.Status == "" {
		req.Status = StatusPending
	}
	if req.SubmittedAt.IsZero() {
		req.SubmittedAt = time.Now()
	}

	q.requests[req.ID] = req
	return nil
}

func (q *Queue) Get(id string) (Request, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	req, exists := q.requests[id]
	if !exists {
		return Request{}, fmt.Errorf("request %q not found", id)
	}
	return req, nil
}

func (q *Queue) Approve(id string, reviewer string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	req, exists := q.requests[id]
	if !exists {
		return fmt.Errorf("request %q not found", id)
	}
	if req.Status != StatusPending {
		return fmt.Errorf("request %q already reviewed (status: %s)", id, req.Status)
	}

	req.Status = StatusApproved
	req.ReviewedBy = reviewer
	req.ReviewedAt = time.Now()
	q.requests[id] = req
	return nil
}

func (q *Queue) Deny(id string, reviewer string, reason string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	req, exists := q.requests[id]
	if !exists {
		return fmt.Errorf("request %q not found", id)
	}
	if req.Status != StatusPending {
		return fmt.Errorf("request %q already reviewed (status: %s)", id, req.Status)
	}

	req.Status = StatusDenied
	req.ReviewedBy = reviewer
	req.ReviewedAt = time.Now()
	req.DenyReason = reason
	q.requests[id] = req
	return nil
}

func (q *Queue) ListPending() []Request {
	q.mu.RLock()
	defer q.mu.RUnlock()

	var result []Request
	for _, req := range q.requests {
		if req.Status == StatusPending {
			result = append(result, req)
		}
	}
	return result
}

func (q *Queue) ListByAgent(agent string) []Request {
	q.mu.RLock()
	defer q.mu.RUnlock()

	var result []Request
	for _, req := range q.requests {
		if req.Agent == agent {
			result = append(result, req)
		}
	}
	return result
}

func (q *Queue) ListAll() []Request {
	q.mu.RLock()
	defer q.mu.RUnlock()

	result := make([]Request, 0, len(q.requests))
	for _, req := range q.requests {
		result = append(result, req)
	}
	return result
}

func (q *Queue) Stats() QueueStats {
	q.mu.RLock()
	defer q.mu.RUnlock()

	stats := QueueStats{Total: len(q.requests)}
	for _, req := range q.requests {
		switch req.Status {
		case StatusPending:
			stats.Pending++
		case StatusApproved:
			stats.Approved++
		case StatusDenied:
			stats.Denied++
		}
	}
	return stats
}
