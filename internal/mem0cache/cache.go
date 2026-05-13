package mem0cache

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"sync"
	"time"
)

// CacheStats reports current cache state.
type CacheStats struct {
	Entries int
	Hits    int64
	Misses  int64
}

type cacheEntry struct {
	value     []byte
	expiresAt time.Time
}

// Cache is a concurrent LRU read cache with TTL eviction.
type Cache struct {
	mu         sync.RWMutex
	entries    map[string]*cacheEntry
	maxEntries int
	defaultTTL time.Duration

	hits   int64
	misses int64

	stopEvict chan struct{}
	done      chan struct{}
}

// CacheConfig controls cache behavior.
type CacheConfig struct {
	MaxEntries    int
	DefaultTTL    time.Duration
	EvictInterval time.Duration
}

func defaultCacheConfig() CacheConfig {
	return CacheConfig{
		MaxEntries:    1000,
		DefaultTTL:    5 * time.Minute,
		EvictInterval: 30 * time.Second,
	}
}

// NewCache creates a cache with the given config.
// A background goroutine evicts expired entries periodically.
func NewCache(cfg CacheConfig) *Cache {
	if cfg.MaxEntries <= 0 {
		cfg.MaxEntries = defaultCacheConfig().MaxEntries
	}
	if cfg.DefaultTTL <= 0 {
		cfg.DefaultTTL = defaultCacheConfig().DefaultTTL
	}
	if cfg.EvictInterval <= 0 {
		cfg.EvictInterval = defaultCacheConfig().EvictInterval
	}

	c := &Cache{
		entries:    make(map[string]*cacheEntry),
		maxEntries: cfg.MaxEntries,
		defaultTTL: cfg.DefaultTTL,
		stopEvict:  make(chan struct{}),
		done:       make(chan struct{}),
	}

	go c.evictLoop(cfg.EvictInterval)
	return c
}

// CacheKey produces a deterministic key from a query and filters.
func CacheKey(query string, filters map[string]interface{}) string {
	h := sha256.New()
	h.Write([]byte(query))
	if filters != nil {
		for k, v := range filters {
			h.Write([]byte(k))
			h.Write([]byte("="))
			switch val := v.(type) {
			case string:
				h.Write([]byte(val))
			case []byte:
				h.Write(val)
			default:
				h.Write([]byte("?"))
			}
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Get retrieves a cached value. Returns nil, false on miss or expiry.
func (c *Cache) Get(key string) ([]byte, bool) {
	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok {
		c.mu.Lock()
		c.misses++
		c.mu.Unlock()
		return nil, false
	}
	if time.Now().After(e.expiresAt) {
		c.mu.Lock()
		delete(c.entries, key)
		c.misses++
		c.mu.Unlock()
		return nil, false
	}

	c.mu.Lock()
	c.hits++
	c.mu.Unlock()

	cp := make([]byte, len(e.value))
	copy(cp, e.value)
	return cp, true
}

// Set stores a value with the given TTL. Zero TTL uses the default.
func (c *Cache) Set(key string, value []byte, ttl time.Duration) {
	if ttl <= 0 {
		ttl = c.defaultTTL
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.entries) >= c.maxEntries {
		c.evictOldest()
	}

	cp := make([]byte, len(value))
	copy(cp, value)
	c.entries[key] = &cacheEntry{
		value:     cp,
		expiresAt: time.Now().Add(ttl),
	}
}

// Stats returns current cache statistics.
func (c *Cache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return CacheStats{
		Entries: len(c.entries),
		Hits:    c.hits,
		Misses:  c.misses,
	}
}

// Stop terminates the background eviction goroutine and waits for it.
func (c *Cache) Stop() {
	close(c.stopEvict)
	<-c.done
}

func (c *Cache) evictLoop(interval time.Duration) {
	defer close(c.done)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopEvict:
			return
		case <-ticker.C:
			c.evictExpired()
		}
	}
}

func (c *Cache) evictExpired() {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()

	evicted := 0
	for k, e := range c.entries {
		if now.After(e.expiresAt) {
			delete(c.entries, k)
			evicted++
		}
	}
	if evicted > 0 {
		slog.Debug("cache evicted expired entries", "count", evicted)
	}
}

// evictOldest removes the entry closest to expiry. Caller must hold c.mu.
func (c *Cache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	first := true

	for k, e := range c.entries {
		if first || e.expiresAt.Before(oldestTime) {
			oldestKey = k
			oldestTime = e.expiresAt
			first = false
		}
	}
	if !first {
		delete(c.entries, oldestKey)
	}
}
