package health

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nfsarch33/helix-dev-tools/internal/config"
)

// cycleRecord is the minimal shape of an EvoLoop cycle JSONL entry.
// Production EvoLoop lines use recorded_at; older tooling may use timestamp.
type cycleRecord struct {
	CycleID    string    `json:"cycle_id"`
	Timestamp  time.Time `json:"timestamp"`
	RecordedAt time.Time `json:"recorded_at"`
	KPIAfter   float64   `json:"kpi_after"`
	Completed  bool      `json:"completed"`
}

func effectiveCycleTime(rec cycleRecord) time.Time {
	if !rec.Timestamp.IsZero() {
		return rec.Timestamp
	}
	return rec.RecordedAt
}

// staleCycleThreshold is the maximum age of the last cycle before warning.
const staleCycleThreshold = 2 * time.Hour

func suiteStaleCycleAge(p config.Paths) *Suite {
	s := &Suite{Name: "EvoLoop Cycle Freshness"}

	logPath := os.Getenv("FLEET_EVOLOOP_CYCLES_PATH")
	if logPath == "" {
		logPath = filepath.Join(p.Home, "fleet", "evoloop-cycles.jsonl")
	}

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		fallback := filepath.Join(p.Home, ".fleet", "evoloop-cycles.jsonl")
		if _, err2 := os.Stat(fallback); err2 == nil {
			logPath = fallback
		} else {
			s.Pass(fmt.Sprintf("no cycle log found (stack may be on WSL): tried %s", logPath))
			return s
		}
	}

	last, total, err := readLastCycle(logPath)
	if err != nil {
		s.Pass(fmt.Sprintf("cycle log unreadable (non-fatal): %v", err))
		return s
	}

	if total == 0 {
		s.Pass("cycle log empty (EvoLoop may not have run yet)")
		return s
	}

	ts := effectiveCycleTime(last)
	if ts.IsZero() {
		s.Pass("last cycle has no usable timestamp — cannot assess freshness")
		return s
	}
	age := time.Since(ts)
	kpiStr := fmt.Sprintf("%.4f", last.KPIAfter)

	if age > staleCycleThreshold {
		s.Fail(
			"last EvoLoop cycle is recent",
			fmt.Sprintf("last cycle %s is %s old (threshold %s), KPI=%s, total=%d — check WSL EvoLoop container",
				last.CycleID, age.Truncate(time.Second), staleCycleThreshold, kpiStr, total),
		)
		return s
	}

	s.Pass(fmt.Sprintf("last cycle %s ago, KPI=%s, total cycles=%d",
		age.Truncate(time.Second), kpiStr, total))
	return s
}

func readLastCycle(path string) (cycleRecord, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return cycleRecord{}, 0, err
	}
	defer f.Close()

	var last cycleRecord
	count := 0
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec cycleRecord
		if err := json.Unmarshal(line, &rec); err == nil {
			last = rec
			count++
		}
	}
	return last, count, scanner.Err()
}
