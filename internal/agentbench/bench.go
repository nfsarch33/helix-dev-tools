package agentbench

import (
	"sync"
	"time"
)

type Config struct {
	Name    string
	AgentID string
}

type Run struct {
	SessionID    string
	PackageCount int
	TestCount    int
	Duration     time.Duration
	PassRate     float64
	Timestamp    time.Time
}

type Comparison struct {
	VelocityDelta float64
	PassRateDelta float64
	Improving     bool
}

type Benchmark struct {
	config Config
	mu     sync.Mutex
	runs   []Run
}

func New(cfg Config) *Benchmark {
	return &Benchmark{config: cfg}
}

func (b *Benchmark) RecordRun(r Run) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if r.Timestamp.IsZero() {
		r.Timestamp = time.Now()
	}
	b.runs = append(b.runs, r)
}

func (b *Benchmark) Runs() []Run {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]Run, len(b.runs))
	copy(out, b.runs)
	return out
}

func (b *Benchmark) Compare() Comparison {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.runs) < 2 {
		return Comparison{}
	}
	prev := b.runs[len(b.runs)-2]
	curr := b.runs[len(b.runs)-1]

	prevVel := velocity(prev)
	currVel := velocity(curr)

	return Comparison{
		VelocityDelta: currVel - prevVel,
		PassRateDelta: curr.PassRate - prev.PassRate,
		Improving:     currVel > prevVel && curr.PassRate >= prev.PassRate,
	}
}

func (b *Benchmark) BestRun() Run {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.runs) == 0 {
		return Run{}
	}
	best := b.runs[0]
	bestScore := score(best)
	for _, r := range b.runs[1:] {
		s := score(r)
		if s > bestScore {
			best = r
			bestScore = s
		}
	}
	return best
}

func (b *Benchmark) VelocityTrend() []float64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	trend := make([]float64, len(b.runs))
	for i, r := range b.runs {
		trend[i] = velocity(r)
	}
	return trend
}

func velocity(r Run) float64 {
	hours := r.Duration.Hours()
	if hours == 0 {
		return 0
	}
	return float64(r.PackageCount) / hours
}

func score(r Run) float64 {
	return velocity(r) * r.PassRate
}
