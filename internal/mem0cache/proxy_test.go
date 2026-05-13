package mem0cache

import (
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func newTestProxy(t *testing.T, upstream UpstreamFunc) *Proxy {
	t.Helper()
	dir := t.TempDir()

	p, err := NewProxy(ProxyConfig{
		Cache: CacheConfig{
			MaxEntries:    100,
			DefaultTTL:    time.Minute,
			EvictInterval: time.Hour,
		},
		Dedup: DedupConfig{
			DBPath: filepath.Join(dir, "dedup.db"),
		},
		Batch: BatchConfig{
			Threshold:     100,
			FlushInterval: time.Hour,
		},
		RateLimiter: RateLimiterConfig{
			AddPerMinute:    60,
			SearchPerMinute: 60,
		},
		Usage: UsageConfig{
			LogPath: filepath.Join(dir, "usage.ndjson"),
		},
		Upstream: upstream,
	})
	if err != nil {
		t.Fatalf("create proxy: %v", err)
	}
	return p
}

func TestProxy_EndToEnd_Search(t *testing.T) {
	var upstreamCalls int64
	p := newTestProxy(t, func(query string, filters map[string]interface{}) ([]byte, error) {
		atomic.AddInt64(&upstreamCalls, 1)
		return []byte(`{"results": []}`), nil
	})
	defer p.Shutdown()

	result, err := p.HandleSearch("test query", nil)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if string(result) != `{"results": []}` {
		t.Fatalf("unexpected result: %s", result)
	}

	result2, err := p.HandleSearch("test query", nil)
	if err != nil {
		t.Fatalf("search 2: %v", err)
	}
	if string(result2) != `{"results": []}` {
		t.Fatalf("unexpected cached result: %s", result2)
	}

	if atomic.LoadInt64(&upstreamCalls) != 1 {
		t.Fatalf("expected 1 upstream call (second should be cached), got %d", atomic.LoadInt64(&upstreamCalls))
	}
}

func TestProxy_EndToEnd_Add(t *testing.T) {
	p := newTestProxy(t, nil)
	defer p.Shutdown()

	if err := p.HandleAdd("first content", nil); err != nil {
		t.Fatalf("add: %v", err)
	}

	if err := p.HandleAdd("first content", nil); err != nil {
		t.Fatalf("add duplicate: %v", err)
	}

	stats := p.Stats()
	if stats.Batch.Pending != 1 {
		t.Fatalf("expected 1 pending (duplicate should be skipped), got %d", stats.Batch.Pending)
	}
	if stats.Dedup.Hits != 1 {
		t.Fatalf("expected 1 dedup hit, got %d", stats.Dedup.Hits)
	}
}

func TestProxy_DedupPlusCacheInteraction(t *testing.T) {
	var upstreamCalls int64
	p := newTestProxy(t, func(query string, _ map[string]interface{}) ([]byte, error) {
		atomic.AddInt64(&upstreamCalls, 1)
		return []byte(`{"found": true}`), nil
	})
	defer p.Shutdown()

	_ = p.HandleAdd("some content", nil)
	_ = p.HandleAdd("some content", nil) // dedup hit

	_, _ = p.HandleSearch("query1", nil)
	_, _ = p.HandleSearch("query1", nil) // cache hit
	_, _ = p.HandleSearch("query2", nil) // cache miss

	stats := p.Stats()
	if stats.AddCalls != 2 {
		t.Fatalf("expected 2 add calls, got %d", stats.AddCalls)
	}
	if stats.SearchCalls != 3 {
		t.Fatalf("expected 3 search calls, got %d", stats.SearchCalls)
	}
	if stats.Cache.Hits != 1 {
		t.Fatalf("expected 1 cache hit, got %d", stats.Cache.Hits)
	}
	if atomic.LoadInt64(&upstreamCalls) != 2 {
		t.Fatalf("expected 2 upstream calls, got %d", atomic.LoadInt64(&upstreamCalls))
	}
}

func TestProxy_ShutdownFlushes(t *testing.T) {
	var flushedCount int64
	dir := t.TempDir()

	p, err := NewProxy(ProxyConfig{
		Cache: CacheConfig{MaxEntries: 10, DefaultTTL: time.Minute, EvictInterval: time.Hour},
		Dedup: DedupConfig{DBPath: filepath.Join(dir, "dedup.db")},
		Batch: BatchConfig{
			Threshold:     100,
			FlushInterval: time.Hour,
			FlushFunc: func(entries []AddEntry) error {
				atomic.AddInt64(&flushedCount, int64(len(entries)))
				return nil
			},
		},
		RateLimiter: RateLimiterConfig{AddPerMinute: 60, SearchPerMinute: 60},
		Usage:       UsageConfig{LogPath: filepath.Join(dir, "usage.ndjson")},
	})
	if err != nil {
		t.Fatalf("create proxy: %v", err)
	}

	_ = p.HandleAdd("content-a", nil)
	_ = p.HandleAdd("content-b", nil)
	p.Shutdown()

	if atomic.LoadInt64(&flushedCount) != 2 {
		t.Fatalf("expected 2 entries flushed on shutdown, got %d", atomic.LoadInt64(&flushedCount))
	}
}

func TestProxy_HandleAdd_RateLimited(t *testing.T) {
	dir := t.TempDir()
	p, err := NewProxy(ProxyConfig{
		Cache:       CacheConfig{MaxEntries: 10, DefaultTTL: time.Minute, EvictInterval: time.Hour},
		Dedup:       DedupConfig{DBPath: filepath.Join(dir, "dedup.db")},
		Batch:       BatchConfig{Threshold: 100, FlushInterval: time.Hour},
		RateLimiter: RateLimiterConfig{AddPerMinute: 6, SearchPerMinute: 60},
		Usage:       UsageConfig{LogPath: filepath.Join(dir, "usage.ndjson")},
	})
	if err != nil {
		t.Fatalf("create proxy: %v", err)
	}
	defer p.Shutdown()

	for i := 0; i < 6; i++ {
		if err := p.HandleAdd(fmt.Sprintf("content-%d", i), nil); err != nil {
			t.Fatalf("add %d: %v", i, err)
		}
	}

	if err := p.HandleAdd("rate-limited-add", nil); err != nil {
		t.Fatalf("rate-limited add should wait and succeed: %v", err)
	}
}

func TestProxy_HandleSearch_RateLimited(t *testing.T) {
	dir := t.TempDir()
	p, err := NewProxy(ProxyConfig{
		Cache:       CacheConfig{MaxEntries: 10, DefaultTTL: time.Minute, EvictInterval: time.Hour},
		Dedup:       DedupConfig{DBPath: filepath.Join(dir, "dedup.db")},
		Batch:       BatchConfig{Threshold: 100, FlushInterval: time.Hour},
		RateLimiter: RateLimiterConfig{AddPerMinute: 60, SearchPerMinute: 6},
		Usage:       UsageConfig{LogPath: filepath.Join(dir, "usage.ndjson")},
		Upstream: func(q string, _ map[string]interface{}) ([]byte, error) {
			return []byte(fmt.Sprintf(`{"q":"%s"}`, q)), nil
		},
	})
	if err != nil {
		t.Fatalf("create proxy: %v", err)
	}
	defer p.Shutdown()

	for i := 0; i < 6; i++ {
		_, err := p.HandleSearch(fmt.Sprintf("query-%d", i), nil)
		if err != nil {
			t.Fatalf("search %d: %v", i, err)
		}
	}

	_, err = p.HandleSearch("rate-limited-search", nil)
	if err != nil {
		t.Fatalf("rate-limited search should wait and succeed: %v", err)
	}
}

func TestProxy_UpstreamError(t *testing.T) {
	p := newTestProxy(t, func(string, map[string]interface{}) ([]byte, error) {
		return nil, fmt.Errorf("connection refused")
	})
	defer p.Shutdown()

	_, err := p.HandleSearch("fail-query", nil)
	if err == nil {
		t.Fatal("expected error from upstream failure")
	}
}

func TestProxy_NilUpstream(t *testing.T) {
	dir := t.TempDir()
	p, err := NewProxy(ProxyConfig{
		Cache:       CacheConfig{MaxEntries: 10, DefaultTTL: time.Minute, EvictInterval: time.Hour},
		Dedup:       DedupConfig{DBPath: filepath.Join(dir, "dedup.db")},
		Batch:       BatchConfig{Threshold: 100, FlushInterval: time.Hour},
		RateLimiter: RateLimiterConfig{AddPerMinute: 60, SearchPerMinute: 60},
		Usage:       UsageConfig{LogPath: filepath.Join(dir, "usage.ndjson")},
	})
	if err != nil {
		t.Fatalf("create proxy: %v", err)
	}
	defer p.Shutdown()

	_, err = p.HandleSearch("query", nil)
	if err == nil {
		t.Fatal("expected error with nil upstream")
	}
}

func TestProxy_Stats(t *testing.T) {
	p := newTestProxy(t, func(string, map[string]interface{}) ([]byte, error) {
		return []byte("ok"), nil
	})
	defer p.Shutdown()

	_ = p.HandleAdd("x", nil)
	_, _ = p.HandleSearch("q", nil)

	stats := p.Stats()
	if stats.AddCalls != 1 {
		t.Fatalf("expected 1 add call, got %d", stats.AddCalls)
	}
	if stats.SearchCalls != 1 {
		t.Fatalf("expected 1 search call, got %d", stats.SearchCalls)
	}
}
