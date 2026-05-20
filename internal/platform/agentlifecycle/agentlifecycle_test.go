package agentlifecycle

import (
	"testing"
	"time"
)

func TestNewAgent(t *testing.T) {
	a := NewAgent("agent-1", "cursor-parent")
	if a.ID != "agent-1" {
		t.Errorf("id: %s", a.ID)
	}
	if a.State != StateRunning {
		t.Errorf("state: %s", a.State)
	}
}

func TestPause(t *testing.T) {
	a := NewAgent("a1", "cursor-parent")
	if err := a.Pause("context limit"); err != nil {
		t.Fatal(err)
	}
	if a.State != StatePaused {
		t.Errorf("state after pause: %s", a.State)
	}
	if a.PauseReason != "context limit" {
		t.Errorf("reason: %s", a.PauseReason)
	}
}

func TestPauseAlreadyPaused(t *testing.T) {
	a := NewAgent("a1", "cursor-parent")
	a.Pause("first")
	if err := a.Pause("second"); err == nil {
		t.Error("expected error pausing already paused agent")
	}
}

func TestResume(t *testing.T) {
	a := NewAgent("a1", "cursor-parent")
	a.Pause("break")
	if err := a.Resume(); err != nil {
		t.Fatal(err)
	}
	if a.State != StateRunning {
		t.Errorf("state after resume: %s", a.State)
	}
	if a.PauseReason != "" {
		t.Errorf("reason should be cleared: %s", a.PauseReason)
	}
}

func TestResumeNotPaused(t *testing.T) {
	a := NewAgent("a1", "cursor-parent")
	if err := a.Resume(); err == nil {
		t.Error("expected error resuming non-paused agent")
	}
}

func TestComplete(t *testing.T) {
	a := NewAgent("a1", "cursor-parent")
	a.Complete("all done")
	if a.State != StateCompleted {
		t.Errorf("state: %s", a.State)
	}
}

func TestSendMessage(t *testing.T) {
	a := NewAgent("a1", "cursor-parent")
	a.SendMessage("hello from parent")
	a.SendMessage("second msg")
	msgs := a.Messages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Body != "hello from parent" {
		t.Errorf("body: %s", msgs[0].Body)
	}
}

func TestMessageOnPausedAgent(t *testing.T) {
	a := NewAgent("a1", "cursor-parent")
	a.Pause("break")
	a.SendMessage("wake up")
	msgs := a.Messages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
}

func TestUptime(t *testing.T) {
	a := NewAgent("a1", "cursor-parent")
	time.Sleep(5 * time.Millisecond)
	if a.Uptime() < 5*time.Millisecond {
		t.Errorf("uptime too low: %v", a.Uptime())
	}
}

func TestTransitionLog(t *testing.T) {
	a := NewAgent("a1", "cursor-parent")
	a.Pause("test")
	a.Resume()
	a.Complete("done")
	log := a.TransitionLog()
	if len(log) != 4 {
		t.Errorf("expected 4 transitions (created+pause+resume+complete), got %d", len(log))
	}
}
