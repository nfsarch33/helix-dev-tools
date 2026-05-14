package resourceguard

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- fake ProcessLister ---

type fakeProcessLister struct {
	procs []ProcessInfo
	err   error
}

func (f *fakeProcessLister) ListProcesses() ([]ProcessInfo, error) {
	return f.procs, f.err
}

// --- tests ---

func TestScan_FindsOrphanProcess(t *testing.T) {
	lister := &fakeProcessLister{
		procs: []ProcessInfo{
			{PID: 100, PPID: 1, Name: "mem0-mcp-go", Elapsed: 30 * time.Minute},
			{PID: 200, PPID: 42, Name: "sentrux", Elapsed: 10 * time.Minute},
		},
	}
	d := NewDetector(lister, DetectorConfig{})

	stale, err := d.Scan()
	require.NoError(t, err)
	require.Len(t, stale, 1)
	assert.Equal(t, 100, stale[0].PID)
	assert.Equal(t, "orphaned (PPID=1)", stale[0].Reason)
}

func TestScan_IgnoresHealthyProcess(t *testing.T) {
	lister := &fakeProcessLister{
		procs: []ProcessInfo{
			{PID: 300, PPID: 42, Name: "sentrux", Elapsed: 30 * time.Minute},
			{PID: 400, PPID: 42, Name: "mem0-mcp-go", Elapsed: 1 * time.Hour},
		},
	}
	d := NewDetector(lister, DetectorConfig{Threshold: DefaultThreshold})

	stale, err := d.Scan()
	require.NoError(t, err)
	assert.Empty(t, stale)
}

func TestScan_RespectsElapsedThreshold(t *testing.T) {
	lister := &fakeProcessLister{
		procs: []ProcessInfo{
			{PID: 500, PPID: 42, Name: "sentrux", Elapsed: 5 * time.Hour},
			{PID: 600, PPID: 42, Name: "pdf-mcp-server", Elapsed: 3 * time.Hour},
		},
	}
	d := NewDetector(lister, DetectorConfig{Threshold: 4 * time.Hour})

	stale, err := d.Scan()
	require.NoError(t, err)
	require.Len(t, stale, 1)
	assert.Equal(t, 500, stale[0].PID)
	assert.Contains(t, stale[0].Reason, "elapsed")
	assert.Contains(t, stale[0].Reason, "threshold")
}

func TestScan_ReturnsEmptyWhenNoStale(t *testing.T) {
	lister := &fakeProcessLister{
		procs: []ProcessInfo{
			{PID: 700, PPID: 42, Name: "vim", Elapsed: 10 * time.Hour},
			{PID: 800, PPID: 42, Name: "zsh", Elapsed: 24 * time.Hour},
		},
	}
	d := NewDetector(lister, DetectorConfig{})

	stale, err := d.Scan()
	require.NoError(t, err)
	assert.Empty(t, stale)
}

func TestScan_IgnoresNonMCPBinaryNames(t *testing.T) {
	lister := &fakeProcessLister{
		procs: []ProcessInfo{
			{PID: 900, PPID: 1, Name: "vim", Elapsed: 1 * time.Hour},
			{PID: 901, PPID: 1, Name: "node", Elapsed: 10 * time.Hour},
		},
	}
	d := NewDetector(lister, DetectorConfig{})

	stale, err := d.Scan()
	require.NoError(t, err)
	assert.Empty(t, stale)
}

func TestScan_CustomBinaryNames(t *testing.T) {
	lister := &fakeProcessLister{
		procs: []ProcessInfo{
			{PID: 1000, PPID: 1, Name: "my-custom-mcp", Elapsed: 1 * time.Hour},
		},
	}
	d := NewDetector(lister, DetectorConfig{
		BinaryNames: []string{"my-custom-mcp"},
	})

	stale, err := d.Scan()
	require.NoError(t, err)
	require.Len(t, stale, 1)
	assert.Equal(t, 1000, stale[0].PID)
}

func TestScan_CustomThreshold(t *testing.T) {
	lister := &fakeProcessLister{
		procs: []ProcessInfo{
			{PID: 1100, PPID: 42, Name: "sentrux", Elapsed: 35 * time.Minute},
		},
	}
	d := NewDetector(lister, DetectorConfig{Threshold: 30 * time.Minute})

	stale, err := d.Scan()
	require.NoError(t, err)
	require.Len(t, stale, 1)
	assert.Contains(t, stale[0].Reason, "elapsed")
}

func TestScan_BothOrphanAndElapsed(t *testing.T) {
	lister := &fakeProcessLister{
		procs: []ProcessInfo{
			{PID: 1200, PPID: 1, Name: "mem0-mcp-go", Elapsed: 10 * time.Hour},
		},
	}
	d := NewDetector(lister, DetectorConfig{Threshold: 4 * time.Hour})

	stale, err := d.Scan()
	require.NoError(t, err)
	// PPID=1 is checked first, so only one entry with orphan reason
	require.Len(t, stale, 1)
	assert.Equal(t, "orphaned (PPID=1)", stale[0].Reason)
}

func TestScan_ListerError(t *testing.T) {
	lister := &fakeProcessLister{err: errors.New("permission denied")}
	d := NewDetector(lister, DetectorConfig{})

	stale, err := d.Scan()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "permission denied")
	assert.Nil(t, stale)
}

func TestScan_EmptyProcessList(t *testing.T) {
	lister := &fakeProcessLister{procs: nil}
	d := NewDetector(lister, DetectorConfig{})

	stale, err := d.Scan()
	require.NoError(t, err)
	assert.Empty(t, stale)
}

func TestScan_SubstringMatch(t *testing.T) {
	lister := &fakeProcessLister{
		procs: []ProcessInfo{
			{PID: 1300, PPID: 1, Name: "/usr/local/bin/mem0-mcp-go", Elapsed: 1 * time.Hour},
		},
	}
	d := NewDetector(lister, DetectorConfig{})

	stale, err := d.Scan()
	require.NoError(t, err)
	require.Len(t, stale, 1)
	assert.Equal(t, 1300, stale[0].PID)
}

func TestDefaultConfig(t *testing.T) {
	lister := &fakeProcessLister{}
	d := NewDetector(lister, DetectorConfig{})

	assert.Equal(t, DefaultThreshold, d.cfg.Threshold)
	assert.Equal(t, KnownMCPBinaries, d.cfg.BinaryNames)
}
