package mem0timeout

import (
	"context"
	"time"
)

type RetryConfig struct {
	MaxAttempts int
	Timeout     time.Duration
}

func RetryWithTimeout(ctx context.Context, fn func(ctx context.Context) error, cfg RetryConfig) error {
	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		tCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
		err := fn(tCtx)
		cancel()

		if err == nil {
			return nil
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}

		if attempt == cfg.MaxAttempts-1 {
			return err
		}
	}
	return nil
}
