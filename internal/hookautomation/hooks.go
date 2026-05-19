package hookautomation

import "sync"

type Config struct {
	HooksPath string
}

type Hook struct {
	Event   string
	Name    string
	Command string
	Enabled bool
}

type Issue struct {
	Hook    Hook
	Message string
}

type Manager struct {
	config Config
	mu     sync.Mutex
	hooks  []Hook
}

func New(cfg Config) *Manager {
	return &Manager{config: cfg}
}

func (m *Manager) Register(h Hook) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hooks = append(m.hooks, h)
}

func (m *Manager) Hooks() []Hook {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Hook, len(m.hooks))
	copy(out, m.hooks)
	return out
}

func (m *Manager) ByEvent(event string) []Hook {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []Hook
	for _, h := range m.hooks {
		if h.Event == event {
			result = append(result, h)
		}
	}
	return result
}

func (m *Manager) Events() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	seen := make(map[string]bool)
	var events []string
	for _, h := range m.hooks {
		if !seen[h.Event] {
			seen[h.Event] = true
			events = append(events, h.Event)
		}
	}
	return events
}

func (m *Manager) Validate() []Issue {
	m.mu.Lock()
	defer m.mu.Unlock()
	var issues []Issue
	seen := make(map[string]bool)
	for _, h := range m.hooks {
		key := h.Event + ":" + h.Name
		if seen[key] {
			issues = append(issues, Issue{Hook: h, Message: "duplicate hook"})
		}
		seen[key] = true
	}
	return issues
}
