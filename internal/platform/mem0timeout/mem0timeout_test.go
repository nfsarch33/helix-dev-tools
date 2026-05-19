package mem0timeout

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetryWithTimeout_SuccessOnFirstTry(t *testing.T) {
	called := 0
	err := RetryWithTimeout(context.Background(), func(ctx context.Context) error {
		called++
		return nil
	}, RetryConfig{MaxAttempts: 3, Timeout: time.Second})

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if called != 1 {
		t.Fatalf("expected 1 call, got %d", called)
	}
}

func TestRetryWithTimeout_RetryOnTimeout(t *testing.T) {
	called := 0
	err := RetryWithTimeout(context.Background(), func(ctx context.Context) error {
		called++
		if called < 3 {
			<-ctx.Done()
			return ctx.Err()
		}
		return nil
	}, RetryConfig{MaxAttempts: 5, Timeout: 10 * time.Millisecond})

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if called != 3 {
		t.Fatalf("expected 3 calls, got %d", called)
	}
}

func TestRetryWithTimeout_MaxAttemptsExceeded(t *testing.T) {
	called := 0
	sentinel := errors.New("persistent failure")
	err := RetryWithTimeout(context.Background(), func(ctx context.Context) error {
		called++
		return sentinel
	}, RetryConfig{MaxAttempts: 3, Timeout: time.Second})

	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}
	if called != 3 {
		t.Fatalf("expected 3 calls, got %d", called)
	}
}

func TestRetryWithTimeout_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	called := 0
	err := RetryWithTimeout(ctx, func(innerCtx context.Context) error {
		called++
		cancel()
		return errors.New("transient")
	}, RetryConfig{MaxAttempts: 5, Timeout: time.Second})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
