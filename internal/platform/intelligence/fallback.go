package intelligence

import (
	"fmt"
	"sync"
	"time"
)

type ProviderState int

const (
	ProviderClosed ProviderState = iota
	ProviderOpen
	ProviderHalfOpen
)

type Provider struct {
	Name     string
	Endpoint string
	CallFn   func(prompt string) (string, error)
}

type CircuitState struct {
	State       ProviderState
	Failures    int
	LastFailure time.Time
	Threshold   int
	ResetAfter  time.Duration
}

type FallbackChain struct {
	mu         sync.RWMutex
	providers  []Provider
	circuits   map[string]*CircuitState
	threshold  int
	resetAfter time.Duration
}

func NewFallbackChain(threshold int, resetAfter time.Duration) *FallbackChain {
	return &FallbackChain{
		circuits:   make(map[string]*CircuitState),
		threshold:  threshold,
		resetAfter: resetAfter,
	}
}

func (f *FallbackChain) AddProvider(p Provider) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.providers = append(f.providers, p)
	f.circuits[p.Name] = &CircuitState{
		Threshold:  f.threshold,
		ResetAfter: f.resetAfter,
	}
}

func (f *FallbackChain) Call(prompt string) (string, error) {
	f.mu.RLock()
	providers := make([]Provider, len(f.providers))
	copy(providers, f.providers)
	f.mu.RUnlock()

	for _, p := range providers {
		if !f.isAvailable(p.Name) {
			continue
		}
		result, err := p.CallFn(prompt)
		if err == nil {
			f.recordSuccess(p.Name)
			return result, nil
		}
		f.recordFailure(p.Name)
	}
	return "", fmt.Errorf("all %d providers exhausted", len(providers))
}

func (f *FallbackChain) isAvailable(name string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	cs := f.circuits[name]
	if cs.State == ProviderOpen {
		if time.Since(cs.LastFailure) > cs.ResetAfter {
			return true
		}
		return false
	}
	return true
}

func (f *FallbackChain) recordFailure(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cs := f.circuits[name]
	cs.Failures++
	cs.LastFailure = time.Now()
	if cs.Failures >= cs.Threshold {
		cs.State = ProviderOpen
	}
}

func (f *FallbackChain) recordSuccess(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cs := f.circuits[name]
	cs.Failures = 0
	cs.State = ProviderClosed
}

func (f *FallbackChain) ProviderCount() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.providers)
}

func (f *FallbackChain) GetState(name string) ProviderState {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if cs, ok := f.circuits[name]; ok {
		return cs.State
	}
	return ProviderClosed
}
