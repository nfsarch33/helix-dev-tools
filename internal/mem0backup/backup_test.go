package mem0backup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeExporter struct {
	err      error
	memories int
}

func (f *fakeExporter) ExportToFile(path string) (int, error) {
	if f.err != nil {
		return 0, f.err
	}
	if err := os.WriteFile(path, []byte(`{"id":"m1","memory":"hello"}`+"\n"), 0o644); err != nil {
		return 0, err
	}
	return f.memories, nil
}

func TestBackup_CreatesCorrectFilename(t *testing.T) {
	dir := t.TempDir()
	mgr := &BackupManager{
		BackupDir: dir,
		Keep:      7,
		Exporter:  &fakeExporter{memories: 3},
		Now:       func() time.Time { return time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC) },
	}

	n, err := mgr.Run()
	require.NoError(t, err)
	assert.Equal(t, 3, n)

	expected := filepath.Join(dir, "mem0-backup-2026-05-15.ndjson")
	_, err = os.Stat(expected)
	assert.NoError(t, err, "backup file should exist")
}

func TestBackup_RotationKeepsExactlyN(t *testing.T) {
	dir := t.TempDir()

	for i := range 10 {
		day := time.Date(2026, 5, 1+i, 0, 0, 0, 0, time.UTC)
		name := fmt.Sprintf("mem0-backup-%s.ndjson", day.Format("2006-01-02"))
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("{}"), 0o644))
	}

	mgr := &BackupManager{
		BackupDir: dir,
		Keep:      7,
		Exporter:  &fakeExporter{memories: 1},
		Now:       func() time.Time { return time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC) },
	}

	_, err := mgr.Run()
	require.NoError(t, err)

	entries, err := filepath.Glob(filepath.Join(dir, "mem0-backup-*.ndjson"))
	require.NoError(t, err)
	assert.Len(t, entries, 7, "should keep exactly 7 backups")
}

func TestBackup_RotationDeletesOldestFirst(t *testing.T) {
	dir := t.TempDir()

	for i := range 10 {
		day := time.Date(2026, 5, 1+i, 0, 0, 0, 0, time.UTC)
		name := fmt.Sprintf("mem0-backup-%s.ndjson", day.Format("2006-01-02"))
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("{}"), 0o644))
	}

	mgr := &BackupManager{
		BackupDir: dir,
		Keep:      7,
		Exporter:  &fakeExporter{memories: 1},
		Now:       func() time.Time { return time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC) },
	}

	_, err := mgr.Run()
	require.NoError(t, err)

	entries, err := filepath.Glob(filepath.Join(dir, "mem0-backup-*.ndjson"))
	require.NoError(t, err)

	sort.Strings(entries)
	oldest := filepath.Base(entries[0])
	assert.Equal(t, "mem0-backup-2026-05-05.ndjson", oldest, "oldest surviving should be May 5 (10 pre-seeded + 1 new = 11, keep 7, delete 4 oldest)")

	newest := filepath.Base(entries[len(entries)-1])
	assert.Equal(t, "mem0-backup-2026-05-15.ndjson", newest, "newest should be today's backup")
}

func TestBackup_HandlesEmptyDir(t *testing.T) {
	dir := t.TempDir()
	mgr := &BackupManager{
		BackupDir: dir,
		Keep:      7,
		Exporter:  &fakeExporter{memories: 2},
		Now:       func() time.Time { return time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC) },
	}

	n, err := mgr.Run()
	require.NoError(t, err)
	assert.Equal(t, 2, n)

	entries, err := filepath.Glob(filepath.Join(dir, "mem0-backup-*.ndjson"))
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}

func TestBackup_ExporterError(t *testing.T) {
	dir := t.TempDir()
	mgr := &BackupManager{
		BackupDir: dir,
		Keep:      7,
		Exporter:  &fakeExporter{err: fmt.Errorf("connection refused")},
		Now:       func() time.Time { return time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC) },
	}

	_, err := mgr.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection refused")

	entries, _ := filepath.Glob(filepath.Join(dir, "mem0-backup-*.ndjson"))
	assert.Empty(t, entries, "no partial backup on error")
}

func TestBackupManager_10DailyExports_Only7Retained(t *testing.T) {
	dir := t.TempDir()

	for day := 1; day <= 10; day++ {
		d := day
		mgr := &BackupManager{
			BackupDir: dir,
			Keep:      7,
			Exporter:  &fakeExporter{memories: d},
			Now:       func() time.Time { return time.Date(2026, 5, d, 0, 0, 0, 0, time.UTC) },
		}
		_, err := mgr.Run()
		require.NoError(t, err, "day %d", day)
	}

	entries, err := filepath.Glob(filepath.Join(dir, "mem0-backup-*.ndjson"))
	require.NoError(t, err)
	assert.Len(t, entries, 7)

	sort.Strings(entries)
	for i, e := range entries {
		expected := fmt.Sprintf("mem0-backup-2026-05-%02d.ndjson", i+4)
		assert.Equal(t, expected, filepath.Base(e), "retained file %d", i)
	}
}

func TestBackupManager_RotationDeletesOldestFirst(t *testing.T) {
	dir := t.TempDir()

	for day := 1; day <= 8; day++ {
		d := day
		mgr := &BackupManager{
			BackupDir: dir,
			Keep:      7,
			Exporter:  &fakeExporter{memories: 1},
			Now:       func() time.Time { return time.Date(2026, 5, d, 0, 0, 0, 0, time.UTC) },
		}
		_, err := mgr.Run()
		require.NoError(t, err)
	}

	entries, err := filepath.Glob(filepath.Join(dir, "mem0-backup-*.ndjson"))
	require.NoError(t, err)
	assert.Len(t, entries, 7)

	_, err = os.Stat(filepath.Join(dir, "mem0-backup-2026-05-01.ndjson"))
	assert.True(t, os.IsNotExist(err), "oldest backup (day 1) should be deleted")

	_, err = os.Stat(filepath.Join(dir, "mem0-backup-2026-05-02.ndjson"))
	assert.NoError(t, err, "day 2 should still exist")

	_, err = os.Stat(filepath.Join(dir, "mem0-backup-2026-05-08.ndjson"))
	assert.NoError(t, err, "newest backup (day 8) should exist")
}

type crashExporter struct{}

func (c *crashExporter) ExportToFile(path string) (int, error) {
	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	f.Write([]byte(`{"id":"partial","memory":"inc`))
	f.Close()
	return 0, fmt.Errorf("simulated crash mid-write")
}

func TestBackupManager_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	mgr := &BackupManager{
		BackupDir: dir,
		Keep:      7,
		Exporter:  &crashExporter{},
		Now:       func() time.Time { return time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC) },
	}

	_, err := mgr.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "simulated crash")

	finals, _ := filepath.Glob(filepath.Join(dir, "mem0-backup-*.ndjson"))
	assert.Empty(t, finals, "no final backup file on crash")

	tmps, _ := filepath.Glob(filepath.Join(dir, "*.tmp"))
	assert.Empty(t, tmps, "no partial .tmp file left behind")
}

func TestBackupManager_ConcurrentBackups(t *testing.T) {
	dir := t.TempDir()

	var wg sync.WaitGroup
	errs := make([]error, 3)

	for i := range 3 {
		d := i + 1
		wg.Add(1)
		go func() {
			defer wg.Done()
			mgr := &BackupManager{
				BackupDir: dir,
				Keep:      7,
				Exporter:  &fakeExporter{memories: d * 10},
				Now:       func() time.Time { return time.Date(2026, 6, d, 0, 0, 0, 0, time.UTC) },
			}
			_, err := mgr.Run()
			errs[d-1] = err
		}()
	}
	wg.Wait()

	for i, err := range errs {
		assert.NoError(t, err, "goroutine %d", i)
	}

	entries, err := filepath.Glob(filepath.Join(dir, "mem0-backup-*.ndjson"))
	require.NoError(t, err)
	assert.Len(t, entries, 3)

	for _, e := range entries {
		data, err := os.ReadFile(e)
		require.NoError(t, err, "read %s", filepath.Base(e))
		assert.NotEmpty(t, data, "%s should not be empty", filepath.Base(e))
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			assert.True(t, json.Valid([]byte(line)), "invalid JSON in %s: %s", filepath.Base(e), line)
		}
	}
}
