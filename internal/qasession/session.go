package qasession

import "sync"

type SessionStatus int

const (
	StatusPending SessionStatus = iota
	StatusRunning
	StatusPassed
	StatusFailed
)

type Config struct {
	SprintID       string
	RepoPath       string
	SentruxEnabled bool
}

type Check struct {
	Name    string
	Command string
}

type CheckResult struct {
	Passed bool
	Output string
}

type Session struct {
	config          Config
	mu              sync.Mutex
	status          SessionStatus
	checks          []Check
	results         map[string]CheckResult
	sentruxBaseline int
	sentruxResult   int
}

func New(cfg Config) *Session {
	return &Session{
		config:  cfg,
		status:  StatusPending,
		results: make(map[string]CheckResult),
	}
}

func (s *Session) Status() SessionStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

func (s *Session) AddCheck(c Check) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checks = append(s.checks, c)
}

func (s *Session) Checks() []Check {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Check, len(s.checks))
	copy(out, s.checks)
	return out
}

func (s *Session) RecordResult(name string, r CheckResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.results[name] = r
}

func (s *Session) Results() map[string]CheckResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]CheckResult, len(s.results))
	for k, v := range s.results {
		out[k] = v
	}
	return out
}

func (s *Session) AllPassed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.checks {
		r, ok := s.results[c.Name]
		if !ok || !r.Passed {
			return false
		}
	}
	return true
}

func (s *Session) SetSentruxBaseline(score int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sentruxBaseline = score
}

func (s *Session) SetSentruxResult(score int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sentruxResult = score
}

func (s *Session) SentruxPassed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sentruxResult >= s.sentruxBaseline
}
