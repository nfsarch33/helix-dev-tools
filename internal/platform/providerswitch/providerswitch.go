package providerswitch

import (
	"fmt"
	"sync"
	"time"
)

type Provider struct {
	Name     string `json:"name"`
	BaseURL  string `json:"base_url"`
	Model    string `json:"model"`
	Priority int    `json:"priority"`
	Healthy  bool   `json:"healthy"`
	LastFail time.Time
}

type Router struct {
	mu        sync.RWMutex
	providers []Provider
	active    int
}

func New(providers []Provider) *Router {
	sorted := make([]Provider, len(providers))
	copy(sorted, providers)
	for i := range sorted {
		sorted[i].Healthy = true
	}
	return &Router{providers: sorted, active: 0}
}

func (r *Router) Active() Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.active >= len(r.providers) {
		return Provider{}
	}
	return r.providers[r.active]
}

func (r *Router) MarkUnhealthy(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.providers {
		if r.providers[i].Name == name {
			r.providers[i].Healthy = false
			r.providers[i].LastFail = time.Now()
			break
		}
	}
	r.selectActive()
}

func (r *Router) MarkHealthy(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.providers {
		if r.providers[i].Name == name {
			r.providers[i].Healthy = true
			break
		}
	}
	r.selectActive()
}

func (r *Router) selectActive() {
	best := -1
	for i, p := range r.providers {
		if p.Healthy {
			if best == -1 || p.Priority > r.providers[best].Priority {
				best = i
			}
		}
	}
	if best >= 0 {
		r.active = best
	}
}

func (r *Router) Failover() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	current := r.providers[r.active].Name
	r.providers[r.active].Healthy = false
	r.providers[r.active].LastFail = time.Now()
	r.selectActive()
	if r.providers[r.active].Name == current && !r.providers[r.active].Healthy {
		return fmt.Errorf("no healthy providers available")
	}
	return nil
}

func (r *Router) List() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Provider, len(r.providers))
	copy(result, r.providers)
	return result
}

func (r *Router) HealthyCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := 0
	for _, p := range r.providers {
		if p.Healthy {
			count++
		}
	}
	return count
}
