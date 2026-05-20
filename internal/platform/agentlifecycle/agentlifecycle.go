package agentlifecycle

import (
	"fmt"
	"sync"
	"time"
)

type State string

const (
	StateRunning   State = "running"
	StatePaused    State = "paused"
	StateCompleted State = "completed"
)

type Message struct {
	Timestamp time.Time `json:"ts"`
	Body      string    `json:"body"`
}

type Transition struct {
	From      State     `json:"from"`
	To        State     `json:"to"`
	Reason    string    `json:"reason,omitempty"`
	Timestamp time.Time `json:"ts"`
}

type Agent struct {
	ID          string    `json:"id"`
	Owner       string    `json:"owner"`
	State       State     `json:"state"`
	PauseReason string    `json:"pause_reason,omitempty"`
	CreatedAt   time.Time `json:"created_at"`

	mu          sync.Mutex
	messages    []Message
	transitions []Transition
}

func NewAgent(id, owner string) *Agent {
	now := time.Now()
	a := &Agent{
		ID:        id,
		Owner:     owner,
		State:     StateRunning,
		CreatedAt: now,
		transitions: []Transition{
			{From: "", To: StateRunning, Reason: "created", Timestamp: now},
		},
	}
	return a
}

func (a *Agent) Pause(reason string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.State != StateRunning {
		return fmt.Errorf("cannot pause agent in state %s", a.State)
	}
	a.transitions = append(a.transitions, Transition{
		From: StateRunning, To: StatePaused, Reason: reason, Timestamp: time.Now(),
	})
	a.State = StatePaused
	a.PauseReason = reason
	return nil
}

func (a *Agent) Resume() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.State != StatePaused {
		return fmt.Errorf("cannot resume agent in state %s", a.State)
	}
	a.transitions = append(a.transitions, Transition{
		From: StatePaused, To: StateRunning, Reason: "resumed", Timestamp: time.Now(),
	})
	a.State = StateRunning
	a.PauseReason = ""
	return nil
}

func (a *Agent) Complete(summary string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.transitions = append(a.transitions, Transition{
		From: a.State, To: StateCompleted, Reason: summary, Timestamp: time.Now(),
	})
	a.State = StateCompleted
}

func (a *Agent) SendMessage(body string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.messages = append(a.messages, Message{Timestamp: time.Now(), Body: body})
}

func (a *Agent) Messages() []Message {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]Message, len(a.messages))
	copy(out, a.messages)
	return out
}

func (a *Agent) Uptime() time.Duration {
	return time.Since(a.CreatedAt)
}

func (a *Agent) TransitionLog() []Transition {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]Transition, len(a.transitions))
	copy(out, a.transitions)
	return out
}
