package health

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nfsarch33/helix-dev-tools/internal/config"
)

func TestSuiteStaleCycleAge_NoFile(t *testing.T) {
	p := config.Paths{Home: t.TempDir()}
	s := suiteStaleCycleAge(p)
	if s.FailCount() > 0 {
		t.Error("should pass when cycle file does not exist")
	}
}

func TestSuiteStaleCycleAge_RecordedAtOnly(t *testing.T) {
	dir := t.TempDir()
	fleetDir := filepath.Join(dir, "fleet")
	if err := os.MkdirAll(fleetDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Production EvoLoop JSONL uses recorded_at, not timestamp.
	recAt := time.Now().Add(-30 * time.Minute).Format(time.RFC3339Nano)
	line := `{"cycle_id":"ev-test","recorded_at":"` + recAt + `","kpi_after":4.4,"completed":true}` + "\n"
	logPath := filepath.Join(fleetDir, "evoloop-cycles.jsonl")
	if err := os.WriteFile(logPath, []byte(line), 0644); err != nil {
		t.Fatal(err)
	}
	p := config.Paths{Home: dir}
	s := suiteStaleCycleAge(p)
	if s.FailCount() > 0 {
		t.Error("should pass when only recorded_at is set and cycle is recent")
	}
}

func TestSuiteStaleCycleAge_RecentCycle(t *testing.T) {
	dir := t.TempDir()
	fleetDir := filepath.Join(dir, "fleet")
	if err := os.MkdirAll(fleetDir, 0755); err != nil {
		t.Fatal(err)
	}

	rec := cycleRecord{
		CycleID:   "cycle-recent",
		Timestamp: time.Now().Add(-30 * time.Minute),
		KPIAfter:  6.5,
		Completed: true,
	}
	data, _ := json.Marshal(rec)

	logPath := filepath.Join(fleetDir, "evoloop-cycles.jsonl")
	if err := os.WriteFile(logPath, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	p := config.Paths{Home: dir}
	s := suiteStaleCycleAge(p)
	if s.FailCount() > 0 {
		t.Error("should pass for recent cycle (30 min old)")
	}
}

func TestSuiteStaleCycleAge_StaleCycle(t *testing.T) {
	dir := t.TempDir()
	fleetDir := filepath.Join(dir, "fleet")
	if err := os.MkdirAll(fleetDir, 0755); err != nil {
		t.Fatal(err)
	}

	rec := cycleRecord{
		CycleID:   "cycle-stale",
		Timestamp: time.Now().Add(-4 * time.Hour),
		KPIAfter:  5.0,
		Completed: true,
	}
	data, _ := json.Marshal(rec)

	logPath := filepath.Join(fleetDir, "evoloop-cycles.jsonl")
	if err := os.WriteFile(logPath, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	p := config.Paths{Home: dir}
	s := suiteStaleCycleAge(p)
	if s.FailCount() == 0 {
		t.Error("should fail for stale cycle (4 hours old)")
	}
}

func TestSuiteStaleCycleAge_EnvOverride(t *testing.T) {
	dir := t.TempDir()
	customDir := filepath.Join(dir, "custom")
	if err := os.MkdirAll(customDir, 0755); err != nil {
		t.Fatal(err)
	}

	rec := cycleRecord{
		CycleID:   "cycle-custom",
		Timestamp: time.Now().Add(-10 * time.Minute),
		KPIAfter:  7.0,
		Completed: true,
	}
	data, _ := json.Marshal(rec)

	logPath := filepath.Join(customDir, "my-cycles.jsonl")
	if err := os.WriteFile(logPath, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("FLEET_EVOLOOP_CYCLES_PATH", logPath)

	p := config.Paths{Home: dir}
	s := suiteStaleCycleAge(p)
	if s.FailCount() > 0 {
		t.Error("should pass for recent custom-path cycle")
	}
}
