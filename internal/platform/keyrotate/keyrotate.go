package keyrotate

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type Key struct {
	Value      string    `json:"value"`
	Label      string    `json:"label"`
	Exhausted  bool      `json:"exhausted"`
	LastUsed   time.Time `json:"last_used"`
	LastFailed time.Time `json:"last_failed,omitempty"`
	UseCount   int64     `json:"use_count"`
}

type Strategy string

const (
	RoundRobin Strategy = "round_robin"
	LeastUsed  Strategy = "least_used"
)

type Pool struct {
	mu       sync.RWMutex
	keys     []*Key
	cursor   atomic.Int64
	strategy Strategy
}

func NewPool(keys []string, labels []string, strategy Strategy) *Pool {
	pool := make([]*Key, len(keys))
	for i, k := range keys {
		label := fmt.Sprintf("key-%d", i+1)
		if i < len(labels) && labels[i] != "" {
			label = labels[i]
		}
		pool[i] = &Key{Value: k, Label: label}
	}
	if strategy == "" {
		strategy = RoundRobin
	}
	return &Pool{keys: pool, strategy: strategy}
}

func (p *Pool) Next() (string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	switch p.strategy {
	case LeastUsed:
		return p.leastUsed()
	default:
		return p.roundRobin()
	}
}

func (p *Pool) roundRobin() (string, error) {
	n := int64(len(p.keys))
	if n == 0 {
		return "", fmt.Errorf("empty key pool")
	}
	for attempts := int64(0); attempts < n; attempts++ {
		idx := p.cursor.Add(1) % n
		key := p.keys[idx]
		if !key.Exhausted {
			key.LastUsed = time.Now()
			key.UseCount++
			return key.Value, nil
		}
	}
	return "", fmt.Errorf("all keys exhausted")
}

func (p *Pool) leastUsed() (string, error) {
	var best *Key
	for _, k := range p.keys {
		if k.Exhausted {
			continue
		}
		if best == nil || k.UseCount < best.UseCount {
			best = k
		}
	}
	if best == nil {
		return "", fmt.Errorf("all keys exhausted")
	}
	best.LastUsed = time.Now()
	best.UseCount++
	return best.Value, nil
}

func (p *Pool) MarkExhausted(value string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, k := range p.keys {
		if k.Value == value {
			k.Exhausted = true
			k.LastFailed = time.Now()
			return
		}
	}
}

func (p *Pool) MarkHealthy(value string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, k := range p.keys {
		if k.Value == value {
			k.Exhausted = false
			return
		}
	}
}

func (p *Pool) ResetAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, k := range p.keys {
		k.Exhausted = false
	}
}

func (p *Pool) HealthyCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	count := 0
	for _, k := range p.keys {
		if !k.Exhausted {
			count++
		}
	}
	return count
}

func (p *Pool) Status() []Key {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]Key, len(p.keys))
	for i, k := range p.keys {
		result[i] = *k
		v := result[i].Value
		if len(v) > 5 {
			v = v[:5] + "***"
		} else {
			v = "***"
		}
		result[i].Value = v
	}
	return result
}

func (p *Pool) Len() int {
	return len(p.keys)
}
