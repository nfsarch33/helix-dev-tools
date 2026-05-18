package approvalqueue

import (
	"testing"
	"time"
)

func TestSubmitAndList(t *testing.T) {
	q := New()

	req := Request{
		ID:          "req-001",
		Agent:       "cursor-parent",
		Action:      "git push --repo global-kb",
		Reason:      "Sprint closeout v6080",
		SubmittedAt: time.Now(),
	}

	err := q.Submit(req)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	pending := q.ListPending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(pending))
	}
	if pending[0].ID != "req-001" {
		t.Errorf("got ID %q", pending[0].ID)
	}
	if pending[0].Status != StatusPending {
		t.Errorf("got status %q", pending[0].Status)
	}
}

func TestApprove(t *testing.T) {
	q := New()
	q.Submit(Request{ID: "r1", Agent: "codex", Action: "push"})

	err := q.Approve("r1", "operator")
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}

	req, _ := q.Get("r1")
	if req.Status != StatusApproved {
		t.Errorf("expected approved, got %q", req.Status)
	}
	if req.ReviewedBy != "operator" {
		t.Errorf("expected reviewer 'operator', got %q", req.ReviewedBy)
	}
	if req.ReviewedAt.IsZero() {
		t.Error("ReviewedAt should be set")
	}
}

func TestDeny(t *testing.T) {
	q := New()
	q.Submit(Request{ID: "r1", Agent: "codex", Action: "force-push"})

	err := q.Deny("r1", "operator", "too risky")
	if err != nil {
		t.Fatalf("Deny: %v", err)
	}

	req, _ := q.Get("r1")
	if req.Status != StatusDenied {
		t.Errorf("expected denied, got %q", req.Status)
	}
	if req.DenyReason != "too risky" {
		t.Errorf("expected reason 'too risky', got %q", req.DenyReason)
	}
}

func TestGetNotFound(t *testing.T) {
	q := New()
	_, err := q.Get("missing")
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestSubmitDuplicate(t *testing.T) {
	q := New()
	q.Submit(Request{ID: "r1", Action: "push"})
	err := q.Submit(Request{ID: "r1", Action: "push"})
	if err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestApproveNotFound(t *testing.T) {
	q := New()
	err := q.Approve("missing", "op")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDenyNotFound(t *testing.T) {
	q := New()
	err := q.Deny("missing", "op", "reason")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestApproveAlreadyReviewed(t *testing.T) {
	q := New()
	q.Submit(Request{ID: "r1", Action: "push"})
	q.Approve("r1", "op")

	err := q.Approve("r1", "op2")
	if err == nil {
		t.Fatal("expected error for already-reviewed request")
	}
}

func TestListByAgent(t *testing.T) {
	q := New()
	q.Submit(Request{ID: "r1", Agent: "cursor-parent", Action: "push"})
	q.Submit(Request{ID: "r2", Agent: "codex", Action: "merge"})
	q.Submit(Request{ID: "r3", Agent: "cursor-parent", Action: "rebase"})

	cursor := q.ListByAgent("cursor-parent")
	if len(cursor) != 2 {
		t.Errorf("expected 2, got %d", len(cursor))
	}
}

func TestListAll(t *testing.T) {
	q := New()
	q.Submit(Request{ID: "r1", Action: "a"})
	q.Submit(Request{ID: "r2", Action: "b"})
	q.Approve("r1", "op")

	all := q.ListAll()
	if len(all) != 2 {
		t.Errorf("expected 2, got %d", len(all))
	}
}

func TestStats(t *testing.T) {
	q := New()
	q.Submit(Request{ID: "r1", Action: "a"})
	q.Submit(Request{ID: "r2", Action: "b"})
	q.Submit(Request{ID: "r3", Action: "c"})
	q.Approve("r1", "op")
	q.Deny("r2", "op", "no")

	stats := q.Stats()
	if stats.Total != 3 {
		t.Errorf("expected total 3, got %d", stats.Total)
	}
	if stats.Pending != 1 {
		t.Errorf("expected 1 pending, got %d", stats.Pending)
	}
	if stats.Approved != 1 {
		t.Errorf("expected 1 approved, got %d", stats.Approved)
	}
	if stats.Denied != 1 {
		t.Errorf("expected 1 denied, got %d", stats.Denied)
	}
}
