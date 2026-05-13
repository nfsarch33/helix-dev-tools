package mem0cache

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestCache_HitMiss(t *testing.T) {
	c := NewCache(CacheConfig{MaxEntries: 10, DefaultTTL: time.Minute, EvictInterval: time.Hour})
	defer c.Stop()

	_, ok := c.Get("missing")
	if ok {
		t.Fatal("expected miss on empty cache")
	}

	c.Set("k1", []byte("hello"), 0)
	val, ok := c.Get("k1")
	if !ok {
		t.Fatal("expected hit after set")
	}
	if string(val) != "hello" {
		t.Fatalf("unexpected value: %s", val)
	}

	stats := c.Stats()
	if stats.Hits != 1 || stats.Misses != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestCache_TTLExpiry(t *testing.T) {
	c := NewCache(CacheConfig{MaxEntries: 10, DefaultTTL: 50 * time.Millisecond, EvictInterval: time.Hour})
	defer c.Stop()

	c.Set("k1", []byte("v1"), 50*time.Millisecond)

	val, ok := c.Get("k1")
	if !ok || string(val) != "v1" {
		t.Fatal("expected hit before expiry")
	}

	time.Sleep(80 * time.Millisecond)

	_, ok = c.Get("k1")
	if ok {
		t.Fatal("expected miss after TTL expiry")
	}
}

func TestCache_MaxEntriesEviction(t *testing.T) {
	c := NewCache(CacheConfig{MaxEntries: 3, DefaultTTL: time.Minute, EvictInterval: time.Hour})
	defer c.Stop()

	c.Set("a", []byte("1"), time.Minute)
	c.Set("b", []byte("2"), time.Minute)
	c.Set("c", []byte("3"), time.Minute)

	stats := c.Stats()
	if stats.Entries != 3 {
		t.Fatalf("expected 3 entries, got %d", stats.Entries)
	}

	c.Set("d", []byte("4"), time.Minute)
	stats = c.Stats()
	if stats.Entries != 3 {
		t.Fatalf("expected 3 entries after eviction, got %d", stats.Entries)
	}
}

func TestCache_ConcurrentAccess(t *testing.T) {
	c := NewCache(CacheConfig{MaxEntries: 100, DefaultTTL: time.Minute, EvictInterval: time.Hour})
	defer c.Stop()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", n)
			c.Set(key, []byte(fmt.Sprintf("val-%d", n)), 0)
			c.Get(key)
		}(i)
	}
	wg.Wait()

	stats := c.Stats()
	if stats.Entries > 100 {
		t.Fatalf("entries exceeded max: %d", stats.Entries)
	}
}

func TestCache_ValueIsolation(t *testing.T) {
	c := NewCache(CacheConfig{MaxEntries: 10, DefaultTTL: time.Minute, EvictInterval: time.Hour})
	defer c.Stop()

	original := []byte("original")
	c.Set("k1", original, 0)

	original[0] = 'X'

	val, ok := c.Get("k1")
	if !ok {
		t.Fatal("expected hit")
	}
	if string(val) != "original" {
		t.Fatalf("cache value was mutated: %s", val)
	}
}

func TestCacheKey_Deterministic(t *testing.T) {
	k1 := CacheKey("query", map[string]interface{}{"a": "1"})
	k2 := CacheKey("query", map[string]interface{}{"a": "1"})
	if k1 != k2 {
		t.Fatal("same inputs should produce same key")
	}

	k3 := CacheKey("other", nil)
	if k1 == k3 {
		t.Fatal("different inputs should produce different keys")
	}
}

func TestCache_DefaultConfig(t *testing.T) {
	c := NewCache(CacheConfig{})
	defer c.Stop()

	c.Set("key", []byte("val"), 0)
	val, ok := c.Get("key")
	if !ok || string(val) != "val" {
		t.Fatal("default config cache should work")
	}
}

func TestCache_EvictLoop(t *testing.T) {
	c := NewCache(CacheConfig{MaxEntries: 10, DefaultTTL: 30 * time.Millisecond, EvictInterval: 50 * time.Millisecond})
	defer c.Stop()

	c.Set("ephemeral", []byte("gone soon"), 30*time.Millisecond)

	time.Sleep(120 * time.Millisecond)

	stats := c.Stats()
	if stats.Entries != 0 {
		t.Fatalf("expected 0 entries after eviction loop, got %d", stats.Entries)
	}
}
