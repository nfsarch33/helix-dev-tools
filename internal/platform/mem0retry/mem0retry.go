package mem0retry

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"time"
)

var ErrMaxRetries = errors.New("max retries exceeded")

type Config struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	JitterPct   float64
}

func DefaultConfig() Config {
	return Config{
		MaxAttempts: 3,
		BaseDelay:   500 * time.Millisecond,
		MaxDelay:    10 * time.Second,
		JitterPct:   0.2,
	}
}

type RetryFunc func(ctx context.Context) error

func Do(ctx context.Context, cfg Config, fn RetryFunc) error {
	var lastErr error
	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		lastErr = fn(ctx)
		if lastErr == nil {
			return nil
		}
		if !IsRetryable(lastErr) {
			return lastErr
		}
		if attempt < cfg.MaxAttempts-1 {
			delay := computeDelay(cfg, attempt)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return errors.Join(ErrMaxRetries, lastErr)
}

func computeDelay(cfg Config, attempt int) time.Duration {
	exp := math.Pow(2, float64(attempt))
	delay := time.Duration(float64(cfg.BaseDelay) * exp)
	if delay > cfg.MaxDelay {
		delay = cfg.MaxDelay
	}
	jitter := time.Duration(float64(delay) * cfg.JitterPct * (rand.Float64()*2 - 1))
	return delay + jitter
}

type RetryableError struct {
	Err        error
	StatusCode int
}

func (e *RetryableError) Error() string { return e.Err.Error() }
func (e *RetryableError) Unwrap() error { return e.Err }

func IsRetryable(err error) bool {
	var re *RetryableError
	if errors.As(err, &re) {
		return re.StatusCode == 502 || re.StatusCode == 503 || re.StatusCode == 429
	}
	return false
}

func NewRetryableError(err error, statusCode int) error {
	return &RetryableError{Err: err, StatusCode: statusCode}
}
