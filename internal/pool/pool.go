package pool

import (
	"context"
	"sync"
)

// Task is a unit of work executed by the pool. It receives a context
// for cancellation and returns an error on failure.
type Task func(ctx context.Context) error

// Pool runs tasks with bounded concurrency. The concurrency limit is
// set at construction and cannot be changed. Tasks that return errors
// are collected; the pool does not short-circuit on the first failure.
type Pool struct {
	concurrency int
}

// New creates a pool with the given concurrency limit. A limit < 1
// is clamped to 1.
func New(concurrency int) *Pool {
	if concurrency < 1 {
		concurrency = 1
	}
	return &Pool{concurrency: concurrency}
}

// Run executes all tasks with bounded concurrency and returns any
// errors collected. The order of errors is non-deterministic.
func (p *Pool) Run(ctx context.Context, tasks []Task) []error {
	if len(tasks) == 0 {
		return nil
	}

	var (
		mu   sync.Mutex
		errs []error
		wg   sync.WaitGroup
		sem  = make(chan struct{}, p.concurrency)
	)

	for _, task := range tasks {
		select {
		case <-ctx.Done():
			mu.Lock()
			errs = append(errs, ctx.Err())
			mu.Unlock()
			return errs
		case sem <- struct{}{}:
		}

		wg.Add(1)
		go func(t Task) {
			defer func() {
				<-sem
				wg.Done()
			}()
			if err := t(ctx); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}(task)
	}

	wg.Wait()
	return errs
}
