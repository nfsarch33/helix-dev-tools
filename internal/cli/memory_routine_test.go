package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nfsarch33/cursor-tools/internal/config"
	"github.com/nfsarch33/cursor-tools/internal/health"
)

func TestRunMemoryRoutine(t *testing.T) {
	oldHome := os.Getenv("HOME")
	home := t.TempDir()
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatal(err)
	}
	defer os.Setenv("HOME", oldHome)

	p := config.DefaultPaths()
	if err := os.MkdirAll(p.HooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, "logs"), 0o755); err != nil {
		t.Fatal(err)
	}

	oldBuild := doctorBuildSuites
	oldRun := doctorRunSuites
	oldRecord := doctorRecordCheckRun
	oldExport := memoryRoutineExportMetrics
	oldParity := memoryRoutineRunParityExport
	oldSync := memoryRoutineSyncDocs
	oldFlags := memoryRoutineFlags
	defer func() {
		doctorBuildSuites = oldBuild
		doctorRunSuites = oldRun
		doctorRecordCheckRun = oldRecord
		memoryRoutineExportMetrics = oldExport
		memoryRoutineRunParityExport = oldParity
		memoryRoutineSyncDocs = oldSync
		memoryRoutineFlags = oldFlags
	}()

	doctorBuildSuites = func(config.Paths, string) []*health.Suite {
		return []*health.Suite{{Name: "dummy"}}
	}
	doctorRunSuites = func(string, []*health.Suite) (int, int) { return 1, 1 }
	doctorRecordCheckRun = func(string, string, string, time.Time, int, int) string { return "run-id" }

	metricsExportCalled := false
	parityExportCalled := false
	syncCalled := false
	memoryRoutineExportMetrics = func(_ config.Paths, days int, exportPath string) error {
		metricsExportCalled = days == 30 && strings.HasSuffix(exportPath, "memory-metrics.md")
		return os.WriteFile(exportPath, []byte("## Memory Layer KPIs"), 0o644)
	}
	memoryRoutineRunParityExport = func(exportPath string) error {
		parityExportCalled = strings.HasSuffix(exportPath, "memory-parity.md")
		return os.WriteFile(exportPath, []byte("Parity proven: `true`\nMissing manifest entries: 0\n"), 0o644)
	}
	memoryRoutineSyncDocs = func(config.Paths) error {
		syncCalled = true
		return nil
	}

	memoryRoutineFlags.days = 30
	memoryRoutineFlags.metricsExport = filepath.Join(home, "logs", "memory-metrics.md")
	memoryRoutineFlags.parityExport = filepath.Join(home, "logs", "memory-parity.md")
	memoryRoutineFlags.skipSync = false

	if err := runMemoryRoutine(nil, nil); err != nil {
		t.Fatalf("runMemoryRoutine() error = %v", err)
	}
	if !metricsExportCalled || !parityExportCalled || !syncCalled {
		t.Fatalf("unexpected routine calls: metrics=%v parity=%v sync=%v", metricsExportCalled, parityExportCalled, syncCalled)
	}
}
