package mem0reindex

import (
	"sync"
	"time"
)

type Config struct {
	Mem0URL    string
	Mem0APIKey string
	BatchSize  int
	Timeout    time.Duration
	UserID     string
	AppID      string
}

type Memory struct {
	ID        string
	Text      string
	HasVector bool
	Metadata  map[string]string
}

type ReindexPlan struct {
	TotalMemories  int
	NeedReindex    int
	AlreadyIndexed int
	BatchCount     int
}

type ReindexResult struct {
	mu           sync.Mutex
	SuccessCount int
	FailCount    int
	Failures     []Failure
	durations    []time.Duration
}

type Failure struct {
	MemoryID string
	Error    string
}

func (r *ReindexResult) RecordSuccess(memID string, dur time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.SuccessCount++
	r.durations = append(r.durations, dur)
}

func (r *ReindexResult) RecordFailure(memID string, errMsg string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.FailCount++
	r.Failures = append(r.Failures, Failure{MemoryID: memID, Error: errMsg})
}

func (r *ReindexResult) TotalDuration() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	var total time.Duration
	for _, d := range r.durations {
		total += d
	}
	return total
}

type Reindexer struct {
	config Config
}

func New(cfg Config) *Reindexer {
	if cfg.BatchSize == 0 {
		cfg.BatchSize = 10
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 90 * time.Second
	}
	return &Reindexer{config: cfg}
}

func (r *Reindexer) Plan(memories []Memory) ReindexPlan {
	need := 0
	indexed := 0
	for _, m := range memories {
		if m.HasVector {
			indexed++
		} else {
			need++
		}
	}
	batchCount := need / r.config.BatchSize
	if need%r.config.BatchSize != 0 {
		batchCount++
	}
	return ReindexPlan{
		TotalMemories:  len(memories),
		NeedReindex:    need,
		AlreadyIndexed: indexed,
		BatchCount:     batchCount,
	}
}

func (r *Reindexer) FilterNeedsReindex(memories []Memory) []Memory {
	var result []Memory
	for _, m := range memories {
		if !m.HasVector {
			result = append(result, m)
		}
	}
	return result
}

func (r *Reindexer) SplitBatches(memories []Memory) [][]Memory {
	var batches [][]Memory
	for i := 0; i < len(memories); i += r.config.BatchSize {
		end := i + r.config.BatchSize
		if end > len(memories) {
			end = len(memories)
		}
		batches = append(batches, memories[i:end])
	}
	return batches
}
