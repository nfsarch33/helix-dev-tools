package mem0cache

import (
	"context"

	"golang.org/x/time/rate"
)

// RateLimiterConfig controls rate limiting for Mem0 operations.
type RateLimiterConfig struct {
	AddPerMinute    int // max add operations per minute (default 5)
	SearchPerMinute int // max search operations per minute (default 10)
}

// RateLimiter provides token-bucket rate limiting for Mem0 API calls.
type RateLimiter struct {
	addLim    *rate.Limiter
	searchLim *rate.Limiter
}

// NewRateLimiter creates a rate limiter from config.
func NewRateLimiter(cfg RateLimiterConfig) *RateLimiter {
	if cfg.AddPerMinute <= 0 {
		cfg.AddPerMinute = 5
	}
	if cfg.SearchPerMinute <= 0 {
		cfg.SearchPerMinute = 10
	}

	return &RateLimiter{
		addLim:    rate.NewLimiter(rate.Limit(float64(cfg.AddPerMinute)/60.0), cfg.AddPerMinute),
		searchLim: rate.NewLimiter(rate.Limit(float64(cfg.SearchPerMinute)/60.0), cfg.SearchPerMinute),
	}
}

// AllowAdd returns true if an add operation is currently allowed.
func (r *RateLimiter) AllowAdd() bool {
	return r.addLim.Allow()
}

// AllowSearch returns true if a search operation is currently allowed.
func (r *RateLimiter) AllowSearch() bool {
	return r.searchLim.Allow()
}

// WaitAdd blocks until an add operation is allowed or ctx expires.
func (r *RateLimiter) WaitAdd(ctx context.Context) error {
	return r.addLim.Wait(ctx)
}

// WaitSearch blocks until a search operation is allowed or ctx expires.
func (r *RateLimiter) WaitSearch(ctx context.Context) error {
	return r.searchLim.Wait(ctx)
}
