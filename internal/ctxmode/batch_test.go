package ctxmode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultTargets_NonEmpty(t *testing.T) {
	targets := DefaultTargets("/fake/home")
	assert.NotEmpty(t, targets)
	for _, tgt := range targets {
		assert.NotEmpty(t, tgt.Path, "target path must not be empty")
		assert.NotEmpty(t, tgt.Source, "target source must not be empty")
	}
}

func TestLoadConfig_ValidJSON(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	cfg := BatchIndexConfig{
		Targets: []IndexTarget{
			{Path: "/tmp/readme.md", Source: "test-readme"},
		},
		TimeoutSec: 15,
	}
	data, err := json.Marshal(cfg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cfgPath, data, 0o644))

	loaded, err := LoadConfig(cfgPath)
	require.NoError(t, err)
	assert.Len(t, loaded.Targets, 1)
	assert.Equal(t, "test-readme", loaded.Targets[0].Source)
	assert.Equal(t, 15, loaded.TimeoutSec)
}

func TestLoadConfig_EmptyTargets(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	require.NoError(t, os.WriteFile(cfgPath, []byte(`{"targets":[]}`), 0o644))

	_, err := LoadConfig(cfgPath)
	assert.ErrorContains(t, err, "no targets")
}

func TestLoadConfig_MissingFile(t *testing.T) {
	_, err := LoadConfig("/nonexistent/config.json")
	assert.ErrorContains(t, err, "read config")
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	require.NoError(t, os.WriteFile(cfgPath, []byte(`{invalid`), 0o644))

	_, err := LoadConfig(cfgPath)
	assert.ErrorContains(t, err, "parse config")
}

func TestRunBatchIndex_MissingFile(t *testing.T) {
	targets := []IndexTarget{
		{Path: "/nonexistent/file.md", Source: "test"},
	}
	results := RunBatchIndex(targets, 5)
	require.Len(t, results, 1)
	assert.Equal(t, "skipped", results[0].Status)
	assert.Contains(t, results[0].Error, "not found")
}

func TestRunBatchIndex_ExistingFileNoContextMode(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.md")
	require.NoError(t, os.WriteFile(f, []byte("# Test"), 0o644))

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", dir)
	defer func() { _ = os.Setenv("PATH", origPath) }()

	targets := []IndexTarget{
		{Path: f, Source: "test-file"},
	}
	results := RunBatchIndex(targets, 5)
	require.Len(t, results, 1)
	assert.Equal(t, "skipped", results[0].Status)
	assert.Contains(t, results[0].Error, "context-mode CLI not on PATH")
}

func TestDefaultConfigPath(t *testing.T) {
	p := DefaultConfigPath("/home/test")
	assert.Equal(t, "/home/test/.cursor/session-index.json", p)
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(home, "test.md"), expandHome("~/test.md"))
	assert.Equal(t, "/abs/path.md", expandHome("/abs/path.md"))
	assert.Equal(t, "relative.md", expandHome("relative.md"))
}
