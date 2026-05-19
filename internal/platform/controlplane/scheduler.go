package controlplane

import (
	"sync"
	"time"
)

type TaskFunc func() error

type ScheduledTask struct {
	Name     string
	Interval time.Duration
	Fn       TaskFunc
	LastRun  time.Time
	LastErr  error
	RunCount int
	Backoff  time.Duration
}

type Scheduler struct {
	mu    sync.RWMutex
	tasks map[string]*ScheduledTask
}

func NewScheduler() *Scheduler {
	return &Scheduler{tasks: make(map[string]*ScheduledTask)}
}

func (s *Scheduler) Register(name string, interval time.Duration, fn TaskFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[name] = &ScheduledTask{
		Name:     name,
		Interval: interval,
		Fn:       fn,
	}
}

func (s *Scheduler) Unregister(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tasks, name)
}

func (s *Scheduler) RunDue() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	var ran []string

	for name, task := range s.tasks {
		effectiveInterval := task.Interval
		if task.Backoff > 0 && task.LastErr != nil {
			effectiveInterval = task.Backoff
		}
		if now.Sub(task.LastRun) >= effectiveInterval {
			err := task.Fn()
			task.LastRun = now
			task.LastErr = err
			task.RunCount++
			if err != nil && task.Backoff == 0 {
				backoff := task.Interval * 2
				if backoff == 0 {
					backoff = 30 * time.Second
				}
				task.Backoff = backoff
			} else if err == nil {
				task.Backoff = 0
			}
			ran = append(ran, name)
		}
	}
	return ran
}

func (s *Scheduler) Status() map[string]ScheduledTask {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]ScheduledTask, len(s.tasks))
	for k, v := range s.tasks {
		result[k] = *v
	}
	return result
}

func (s *Scheduler) TaskCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.tasks)
}
