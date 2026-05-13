package mem0cache

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// UpstreamFunc performs the actual Mem0 search API call.
type UpstreamFunc func(query string, filters map[string]interface{}) ([]byte, error)

// ProxyConfig wires all components together.
type ProxyConfig struct {
	Cache       CacheConfig
	Dedup       DedupConfig
	Batch       BatchConfig
	RateLimiter RateLimiterConfig
	Usage       UsageConfig
	Upstream    UpstreamFunc // called for cache-miss searches
}

// ProxyStats aggregates stats from all sub-components.
type ProxyStats struct {
	Cache       CacheStats
	Dedup       DedupStats
	Batch       BatchStats
	AddCalls    int64
	SearchCalls int64
}

// Proxy orchestrates cache, dedup, batch, rate limiting, and usage tracking.
type Proxy struct {
	cache       *Cache
	dedup       *Dedup
	batch       *Batch
	rateLimiter *RateLimiter
	usage       *UsageTracker
	upstream    UpstreamFunc

	addCalls    int64
	searchCalls int64
}

// NewProxy initializes all sub-components and returns a ready Proxy.
func NewProxy(cfg ProxyConfig) (*Proxy, error) {
	cache := NewCache(cfg.Cache)

	dedup, err := NewDedup(cfg.Dedup)
	if err != nil {
		cache.Stop()
		return nil, fmt.Errorf("init dedup: %w", err)
	}

	usage, err := NewUsageTracker(cfg.Usage)
	if err != nil {
		cache.Stop()
		dedup.Close()
		return nil, fmt.Errorf("init usage tracker: %w", err)
	}

	p := &Proxy{
		cache:       cache,
		dedup:       dedup,
		rateLimiter: NewRateLimiter(cfg.RateLimiter),
		usage:       usage,
		upstream:    cfg.Upstream,
	}

	batchCfg := cfg.Batch
	if batchCfg.FlushFunc == nil {
		batchCfg.FlushFunc = func(entries []AddEntry) error {
			usage.LogFlush(len(entries))
			slog.Info("proxy batch flushed", "count", len(entries))
			return nil
		}
	}
	p.batch = NewBatch(batchCfg)

	if p.upstream == nil {
		p.upstream = func(string, map[string]interface{}) ([]byte, error) {
			return nil, fmt.Errorf("no upstream configured")
		}
	}

	return p, nil
}

// HandleAdd processes an add request through dedup -> rate limit -> batch.
func (p *Proxy) HandleAdd(content string, metadata map[string]interface{}) error {
	p.addCalls++

	if p.dedup.IsDuplicate(content) {
		p.usage.LogAdd(true)
		slog.Debug("proxy add skipped (duplicate)")
		return nil
	}

	if !p.rateLimiter.AllowAdd() {
		p.usage.LogRateLimit("add")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := p.rateLimiter.WaitAdd(ctx); err != nil {
			return fmt.Errorf("rate limit wait: %w", err)
		}
	}

	if err := p.dedup.Mark(content); err != nil {
		slog.Warn("dedup mark failed", "err", err)
	}

	p.batch.Add(AddEntry{
		Content:  content,
		Metadata: metadata,
		AddedAt:  time.Now(),
	})

	p.usage.LogAdd(false)
	return nil
}

// HandleSearch processes a search through cache -> rate limit -> upstream -> cache store.
func (p *Proxy) HandleSearch(query string, filters map[string]interface{}) ([]byte, error) {
	p.searchCalls++
	key := CacheKey(query, filters)

	if val, ok := p.cache.Get(key); ok {
		p.usage.LogSearch(true)
		return val, nil
	}

	if !p.rateLimiter.AllowSearch() {
		p.usage.LogRateLimit("search")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := p.rateLimiter.WaitSearch(ctx); err != nil {
			return nil, fmt.Errorf("rate limit wait: %w", err)
		}
	}

	result, err := p.upstream(query, filters)
	if err != nil {
		p.usage.LogSearch(false)
		return nil, fmt.Errorf("upstream search: %w", err)
	}

	p.cache.Set(key, result, 0)
	p.usage.LogSearch(false)
	return result, nil
}

// Shutdown gracefully stops all components.
func (p *Proxy) Shutdown() {
	p.batch.Shutdown()
	p.cache.Stop()
	if err := p.dedup.Close(); err != nil {
		slog.Warn("dedup close error", "err", err)
	}
	if err := p.usage.Close(); err != nil {
		slog.Warn("usage close error", "err", err)
	}
	slog.Info("proxy shutdown complete", "stats", p.Stats())
}

// Stats returns aggregated statistics from all components.
func (p *Proxy) Stats() ProxyStats {
	return ProxyStats{
		Cache:       p.cache.Stats(),
		Dedup:       p.dedup.Stats(),
		Batch:       p.batch.Stats(),
		AddCalls:    p.addCalls,
		SearchCalls: p.searchCalls,
	}
}
