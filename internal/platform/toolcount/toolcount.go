package toolcount

import "sync"

type Counter struct {
	mu     sync.Mutex
	counts map[string]int
}

func New() *Counter {
	return &Counter{counts: make(map[string]int)}
}

func (c *Counter) Increment(tool string) {
	c.mu.Lock()
	c.counts[tool]++
	c.mu.Unlock()
}

func (c *Counter) Report() map[string]int {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make(map[string]int, len(c.counts))
	for k, v := range c.counts {
		result[k] = v
	}
	return result
}

func (c *Counter) Total() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	total := 0
	for _, v := range c.counts {
		total += v
	}
	return total
}
