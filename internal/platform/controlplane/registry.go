package controlplane

import (
	"sync"
	"time"
)

type ServiceStatus int

const (
	StatusHealthy ServiceStatus = iota
	StatusDegraded
	StatusDown
)

type ServiceEntry struct {
	Name     string
	Endpoint string
	Status   ServiceStatus
	TTL      time.Duration
	LastSeen time.Time
	Metadata map[string]string
}

type Registry struct {
	mu       sync.RWMutex
	services map[string]*ServiceEntry
}

func NewRegistry() *Registry {
	return &Registry{services: make(map[string]*ServiceEntry)}
}

func (r *Registry) Register(entry ServiceEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry.LastSeen = time.Now()
	r.services[entry.Name] = &entry
}

func (r *Registry) Deregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.services, name)
}

func (r *Registry) Get(name string) (ServiceEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.services[name]
	if !ok {
		return ServiceEntry{}, false
	}
	return *e, true
}

func (r *Registry) Healthy() []ServiceEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []ServiceEntry
	now := time.Now()
	for _, e := range r.services {
		if e.TTL > 0 && now.Sub(e.LastSeen) > e.TTL {
			continue
		}
		if e.Status == StatusHealthy {
			result = append(result, *e)
		}
	}
	return result
}

func (r *Registry) ExpireTTL() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	expired := 0
	for name, e := range r.services {
		if e.TTL > 0 && now.Sub(e.LastSeen) > e.TTL {
			delete(r.services, name)
			expired++
		}
	}
	return expired
}

func (r *Registry) Heartbeat(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.services[name]
	if !ok {
		return false
	}
	e.LastSeen = time.Now()
	return true
}

func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.services)
}
