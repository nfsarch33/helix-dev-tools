package loadtest

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type Config struct {
	Concurrent int
	Requests   int
	TargetFn   func() error
}

type Result struct {
	Total       int
	Succeeded   int
	Failed      int
	DurationMS  int64
	P95MS       int64
}

func Run(cfg Config) Result {
	// Ensure at least 1 concurrent worker
	if cfg.Concurrent < 1 {
		cfg.Concurrent = 1
	}

	// Compute requests per worker
	requestsPerWorker := cfg.Requests / cfg.Concurrent
	remainder := cfg.Requests % cfg.Concurrent

	start := time.Now()
	var wg sync.WaitGroup
	var totalSucceeded, totalFailed int64
	latencies := make([]int64, 0, cfg.Requests)
	var mu sync.Mutex

	for i := 0; i < cfg.Concurrent; i++ {
		workerRequests := requestsPerWorker
		if i < remainder {
			workerRequests++
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			for j := 0; j < workerRequests; j++ {
				workerStart := time.Now()
				err := cfg.TargetFn()
				latency := time.Since(workerStart).Milliseconds()

				mu.Lock()
				latencies = append(latencies, latency)
				mu.Unlock()

				if err != nil {
					atomic.AddInt64(&totalFailed, 1)
				} else {
					atomic.AddInt64(&totalSucceeded, 1)
				}
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)
	duration := elapsed.Milliseconds()
	if duration == 0 && cfg.Requests > 0 {
		duration = 1
	}

	// Compute P95 latency
	mu.Lock()
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	p95Index := len(latencies) * 95 / 100
	p95 := latencies[p95Index]
	mu.Unlock()

	return Result{
		Total:       cfg.Requests,
		Succeeded:   int(totalSucceeded),
		Failed:      int(totalFailed),
		DurationMS:  duration,
		P95MS:       p95,
	}
}