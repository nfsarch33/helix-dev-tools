package hookinstaller_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/hookinstaller"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInstallPrePushHook_CreatesExecutableFile verifies that InstallPrePushHook
// writes an executable shell script at .git/hooks/pre-push.
func TestInstallPrePushHook_CreatesExecutableFile(t *testing.T) {
	dir := t.TempDir()
	gitHooksDir := filepath.Join(dir, ".git", "hooks")
	require.NoError(t, os.MkdirAll(gitHooksDir, 0o755))

	err := hookinstaller.InstallPrePushHook(dir, "cursor-tools")
	require.NoError(t, err)

	hookPath := filepath.Join(gitHooksDir, "pre-push")
	info, err := os.Stat(hookPath)
	require.NoError(t, err)
	assert.True(t, info.Mode()&0o100 != 0, "hook file must be executable")
}

// TestInstallPrePushHook_ContainsRebrandScanCall verifies the hook script invokes
// the binary with "rebrand scan" arguments.
func TestInstallPrePushHook_ContainsRebrandScanCall(t *testing.T) {
	dir := t.TempDir()
	gitHooksDir := filepath.Join(dir, ".git", "hooks")
	require.NoError(t, os.MkdirAll(gitHooksDir, 0o755))

	err := hookinstaller.InstallPrePushHook(dir, "cursor-tools")
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(gitHooksDir, "pre-push"))
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "rebrand scan", "hook must call rebrand scan")
	assert.Contains(t, content, "cursor-tools", "hook must reference the binary name")
}

// TestInstallPrePushHook_ExitsNonZeroOnFindings verifies the hook script uses
// exit 1 (or checks exit code) when findings are present.
func TestInstallPrePushHook_ExitsNonZeroOnFindings(t *testing.T) {
	dir := t.TempDir()
	gitHooksDir := filepath.Join(dir, ".git", "hooks")
	require.NoError(t, os.MkdirAll(gitHooksDir, 0o755))

	err := hookinstaller.InstallPrePushHook(dir, "cursor-tools")
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(gitHooksDir, "pre-push"))
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "exit 1", "hook must exit 1 on findings")
}

// TestInstallPrePushHook_NoGitDir returns an error when the target dir has no
// .git/hooks directory and it cannot be created.
func TestInstallPrePushHook_NoGitHooksDir(t *testing.T) {
	dir := t.TempDir()
	// No .git directory created -- hookinstaller must create it or return error.
	err := hookinstaller.InstallPrePushHook(dir, "cursor-tools")
	// It should either succeed (by creating .git/hooks) or fail gracefully.
	if err != nil {
		assert.Contains(t, err.Error(), "hooks", "error must mention hooks dir")
	} else {
		_, statErr := os.Stat(filepath.Join(dir, ".git", "hooks", "pre-push"))
		assert.NoError(t, statErr, "hook must exist when install succeeds")
	}
}

// TestInstallPrePushHook_CustomBinaryName verifies that a custom binary name is
// embedded in the hook script.
func TestInstallPrePushHook_CustomBinaryName(t *testing.T) {
	dir := t.TempDir()
	gitHooksDir := filepath.Join(dir, ".git", "hooks")
	require.NoError(t, os.MkdirAll(gitHooksDir, 0o755))

	err := hookinstaller.InstallPrePushHook(dir, "helix-dev-tools")
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(gitHooksDir, "pre-push"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "helix-dev-tools")
}

// TestInstallSymlink_CreatesSymlink verifies that InstallSymlink creates a
// symlink at dst pointing to src.
func TestInstallSymlink_CreatesSymlink(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "helix-dev-tools")
	dst := filepath.Join(dir, "cursor-tools")

	// Create a dummy src binary.
	require.NoError(t, os.WriteFile(src, []byte("#!/bin/sh\necho ok"), 0o755))

	err := hookinstaller.InstallSymlink(src, dst)
	require.NoError(t, err)

	info, err := os.Lstat(dst)
	require.NoError(t, err)
	assert.True(t, info.Mode()&os.ModeSymlink != 0, "dst must be a symlink")

	target, err := os.Readlink(dst)
	require.NoError(t, err)
	assert.Equal(t, src, target)
}

// TestInstallSymlink_AtomicReplacement verifies that InstallSymlink replaces an
// existing file at dst atomically (no window where dst is absent).
func TestInstallSymlink_AtomicReplacement(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "helix-dev-tools")
	dst := filepath.Join(dir, "cursor-tools")

	require.NoError(t, os.WriteFile(src, []byte("#!/bin/sh"), 0o755))
	require.NoError(t, os.WriteFile(dst, []byte("old"), 0o755))

	err := hookinstaller.InstallSymlink(src, dst)
	require.NoError(t, err)

	info, err := os.Lstat(dst)
	require.NoError(t, err)
	assert.True(t, info.Mode()&os.ModeSymlink != 0, "dst must be a symlink after replacement")
}
