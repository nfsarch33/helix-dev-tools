package cli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestMem0WatchdogCmd_Registered(t *testing.T) {
	t.Parallel()
	for _, sub := range rootCmd.Commands() {
		if sub.Name() == "mem0-watchdog" {
			return
		}
	}
	t.Fatal("mem0-watchdog subcommand not registered on rootCmd")
}

func TestMem0WatchdogCmd_OnceOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	mem0WatchdogProbeURL = srv.URL
	mem0WatchdogTimeout = 2 * time.Second
	mem0WatchdogInterval = 60 * time.Second
	mem0WatchdogThreshold = 3
	mem0WatchdogLogPath = filepath.Join(t.TempDir(), "mw.ndjson")
	mem0WatchdogPortPat = "18888"
	mem0WatchdogOnce = true
	mem0WatchdogKill = false
	defer func() { mem0WatchdogOnce = false }()

	out := &bytes.Buffer{}
	mem0WatchdogCmd.SetOut(out)
	mem0WatchdogCmd.SetErr(out)
	mem0WatchdogCmd.SetContext(context.Background())

	if err := runMem0Watchdog(mem0WatchdogCmd, nil); err != nil {
		t.Fatalf("expected ok, got %v (out=%q)", err, out.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("probe ok")) {
		t.Fatalf("expected probe ok in output, got %q", out.String())
	}
}

func TestMem0WatchdogCmd_OnceFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	mem0WatchdogProbeURL = srv.URL
	mem0WatchdogTimeout = 2 * time.Second
	mem0WatchdogLogPath = filepath.Join(t.TempDir(), "mw.ndjson")
	mem0WatchdogOnce = true
	mem0WatchdogKill = false
	defer func() { mem0WatchdogOnce = false }()

	out := &bytes.Buffer{}
	mem0WatchdogCmd.SetOut(out)
	mem0WatchdogCmd.SetErr(out)
	mem0WatchdogCmd.SetContext(context.Background())

	err := runMem0Watchdog(mem0WatchdogCmd, nil)
	if err == nil {
		t.Fatalf("expected error, got nil (out=%q)", out.String())
	}
}
