package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/nfsarch33/cursor-tools/internal/config"
)

func resolveSelfBinary(paths config.Paths) (string, error) {
	binPath := filepath.Join(paths.BinDir, "cursor-tools")
	if _, err := os.Stat(binPath); err == nil {
		return binPath, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	base := filepath.Base(exe)
	if base != "cursor-tools" && !strings.HasPrefix(base, "cursor-tools") {
		return "", fmt.Errorf("cursor-tools binary not found")
	}
	return exe, nil
}

func runCommandOutput(timeout time.Duration, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

func runCommandStream(timeout time.Duration, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func runSelfCommandOutput(timeout time.Duration, paths config.Paths, args ...string) ([]byte, error) {
	binPath, err := resolveSelfBinary(paths)
	if err != nil {
		return nil, err
	}
	return runCommandOutput(timeout, binPath, args...)
}

func runSelfCommandStream(timeout time.Duration, paths config.Paths, args ...string) error {
	binPath, err := resolveSelfBinary(paths)
	if err != nil {
		return err
	}
	return runCommandStream(timeout, binPath, args...)
}
