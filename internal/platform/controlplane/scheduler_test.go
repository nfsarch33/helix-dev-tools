package controlplane

import (
	"errors"
	"testing"
	"time"
)

func TestScheduler_RegisterAndRun(t *testing.T) {
	s := NewScheduler()
	called := 0
	s.Register("task1", 0, func() error { called++; return nil })

	ran := s.RunDue()
	if len(ran) != 1 || ran[0] != "task1" {
		t.Errorf("expected task1 to run, got %v", ran)
	}
	if called != 1 {
		t.Errorf("expected 1 call, got %d", called)
	}
}

func TestScheduler_NotDueYet(t *testing.T) {
	s := NewScheduler()
	s.Register("slow", 1*time.Hour, func() error { return nil })

	s.RunDue()
	ran := s.RunDue()
	if len(ran) != 0 {
		t.Error("task should not run twice within interval")
	}
}

func TestScheduler_BackoffOnError(t *testing.T) {
	s := NewScheduler()
	attempts := 0
	s.Register("flaky", 0, func() error {
		attempts++
		return errors.New("fail")
	})

	s.RunDue()
	status := s.Status()
	task := status["flaky"]
	if task.Backoff == 0 {
		t.Error("expected backoff after error")
	}
	if task.LastErr == nil {
		t.Error("expected error recorded")
	}
}

func TestScheduler_BackoffResetOnSuccess(t *testing.T) {
	s := NewScheduler()
	fail := true
	s.Register("recover", 0, func() error {
		if fail {
			fail = false
			return errors.New("oops")
		}
		return nil
	})

	s.RunDue()
	s.mu.Lock()
	s.tasks["recover"].LastRun = time.Time{}
	s.mu.Unlock()
	s.RunDue()

	status := s.Status()
	if status["recover"].Backoff != 0 {
		t.Error("expected backoff reset after success")
	}
}

func TestScheduler_Unregister(t *testing.T) {
	s := NewScheduler()
	s.Register("temp", 0, func() error { return nil })
	s.Unregister("temp")

	if s.TaskCount() != 0 {
		t.Error("expected 0 tasks after unregister")
	}
}

func TestScheduler_TaskCount(t *testing.T) {
	s := NewScheduler()
	s.Register("a", 0, func() error { return nil })
	s.Register("b", 0, func() error { return nil })

	if s.TaskCount() != 2 {
		t.Errorf("expected 2, got %d", s.TaskCount())
	}
}
