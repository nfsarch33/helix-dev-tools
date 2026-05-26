// runx-public-repo-gate: allow-file fleet_host_alias
// runx-public-repo-gate: allow-file network_topology
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func mockExecOK(_ context.Context, _ string, _ ...string) ([]byte, error) {
	return []byte("OK"), nil
}

func mockExecFail(_ context.Context, _ string, _ ...string) ([]byte, error) {
	return nil, fmt.Errorf("connection refused")
}

func mockExecCustom(output string) func(context.Context, string, ...string) ([]byte, error) {
	return func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return []byte(output), nil
	}
}

func defaultTestConfig() FleetConfig {
	return FleetConfig{
		SSHTarget:        "test-host",
		EngramHealthzURL: "http://127.0.0.1:8280/healthz",
		EngramTunnelURL:  "http://127.0.0.1:18888/healthz",
		DashboardURL:     "http://127.0.0.1:9095/api/health",
	}
}

func TestBuildFleetProbes_Count_Remote(t *testing.T) {
	probes := buildFleetProbes(defaultTestConfig())
	if len(probes) != 11 {
		t.Fatalf("expected 11 probes in remote mode, got %d", len(probes))
	}
}

func TestBuildFleetProbes_Count_Local(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.LocalMode = true
	probes := buildFleetProbes(cfg)
	if len(probes) != 9 {
		t.Fatalf("expected 9 probes in local mode (no SSH/tunnel), got %d", len(probes))
	}
}

func TestBuildFleetProbes_SSHTarget(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.SSHTarget = "custom-host"
	probes := buildFleetProbes(cfg)
	for _, p := range probes {
		if p.Local {
			continue
		}
		found := false
		for _, arg := range p.Command {
			if arg == "custom-host" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("probe %q does not use custom SSH target", p.Name)
		}
	}
}

func TestBuildFleetProbes_LocalMode_UsesShellDirect(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.LocalMode = true
	probes := buildFleetProbes(cfg)
	for _, p := range probes {
		if p.Command[0] != "sh" {
			t.Errorf("local probe %q should use sh, got %s", p.Name, p.Command[0])
		}
		if p.Command[1] != "-c" {
			t.Errorf("local probe %q should have -c arg, got %s", p.Name, p.Command[1])
		}
	}
}

func TestBuildFleetProbes_LocalMode_NoSSHProbe(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.LocalMode = true
	probes := buildFleetProbes(cfg)
	for _, p := range probes {
		if p.Name == "SSH connectivity" {
			t.Error("local mode should not include SSH connectivity probe")
		}
		if p.Name == "Engram tunnel" {
			t.Error("local mode should not include Engram tunnel probe")
		}
	}
}

func TestIsLocalMode_EnvVar(t *testing.T) {
	old := doctorFleetFlags.local
	doctorFleetFlags.local = false
	defer func() { doctorFleetFlags.local = old }()

	t.Setenv("FLEET_LOCAL", "true")
	if !isLocalMode() {
		t.Error("expected local mode when FLEET_LOCAL=true")
	}

	t.Setenv("FLEET_LOCAL", "false")
	if isLocalMode() {
		t.Error("expected remote mode when FLEET_LOCAL=false")
	}
}

func TestIsLocalMode_Flag(t *testing.T) {
	old := doctorFleetFlags.local
	doctorFleetFlags.local = true
	defer func() { doctorFleetFlags.local = old }()

	t.Setenv("FLEET_LOCAL", "")
	if !isLocalMode() {
		t.Error("expected local mode when --local flag is set")
	}
}

func TestWrapCommand_Remote(t *testing.T) {
	cfg := FleetConfig{SSHTarget: "test-host", LocalMode: false}
	cmd := wrapCommand(cfg, "echo hello")
	if cmd[0] != "runx" {
		t.Errorf("remote mode should use runx, got %s", cmd[0])
	}
	if cmd[len(cmd)-1] != "echo hello" {
		t.Errorf("raw command should be last arg, got %s", cmd[len(cmd)-1])
	}
}

func TestWrapCommand_Local(t *testing.T) {
	cfg := FleetConfig{SSHTarget: "test-host", LocalMode: true}
	cmd := wrapCommand(cfg, "echo hello")
	expected := []string{"sh", "-c", "echo hello"}
	if len(cmd) != 3 || cmd[0] != expected[0] || cmd[1] != expected[1] || cmd[2] != expected[2] {
		t.Errorf("local mode should use sh -c, got %v", cmd)
	}
}

func TestBuildFleetProbes_LocalProbe_RemoteMode(t *testing.T) {
	probes := buildFleetProbes(defaultTestConfig())
	localCount := 0
	for _, p := range probes {
		if p.Local {
			localCount++
			if p.Command[0] != "curl" {
				t.Errorf("local probe %q should use curl, got %s", p.Name, p.Command[0])
			}
		}
	}
	if localCount != 1 {
		t.Fatalf("expected 1 local probe in remote mode, got %d", localCount)
	}
}

func TestBuildFleetProbes_AllHaveTimeout(t *testing.T) {
	probes := buildFleetProbes(defaultTestConfig())
	for _, p := range probes {
		if p.Timeout == 0 {
			t.Errorf("probe %q has zero timeout", p.Name)
		}
	}
}

func TestBuildFleetProbes_AllHaveNonEmptyExpect(t *testing.T) {
	probes := buildFleetProbes(defaultTestConfig())
	for _, p := range probes {
		if p.Expect == "" {
			t.Errorf("probe %q has empty Expect", p.Name)
		}
	}
}

func TestRunSingleFleetProbe_Green(t *testing.T) {
	old := fleetExecCommandContext
	fleetExecCommandContext = mockExecOK
	defer func() { fleetExecCommandContext = old }()

	probe := FleetProbe{
		Name:    "test-probe",
		Command: []string{"echo", "OK"},
		Expect:  "OK",
		Timeout: 5 * time.Second,
	}
	result := runSingleFleetProbe(probe)
	if result.Status != FleetGreen {
		t.Fatalf("expected GREEN, got %s", result.Status)
	}
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
}

func TestRunSingleFleetProbe_Red(t *testing.T) {
	old := fleetExecCommandContext
	fleetExecCommandContext = mockExecFail
	defer func() { fleetExecCommandContext = old }()

	probe := FleetProbe{
		Name:    "failing-probe",
		Command: []string{"false"},
		Expect:  "OK",
		Timeout: 5 * time.Second,
	}
	result := runSingleFleetProbe(probe)
	if result.Status != FleetRed {
		t.Fatalf("expected RED, got %s", result.Status)
	}
	if result.Error == "" {
		t.Fatal("expected non-empty error")
	}
}

func TestRunSingleFleetProbe_Yellow(t *testing.T) {
	old := fleetExecCommandContext
	fleetExecCommandContext = mockExecCustom("Pending")
	defer func() { fleetExecCommandContext = old }()

	probe := FleetProbe{
		Name:    "yellow-probe",
		Command: []string{"echo", "Pending"},
		Expect:  "Ready",
		Timeout: 5 * time.Second,
	}
	result := runSingleFleetProbe(probe)
	if result.Status != FleetYellow {
		t.Fatalf("expected YELLOW, got %s", result.Status)
	}
}

func TestRunSingleFleetProbe_Duration(t *testing.T) {
	old := fleetExecCommandContext
	fleetExecCommandContext = mockExecOK
	defer func() { fleetExecCommandContext = old }()

	probe := FleetProbe{
		Name:    "timing-probe",
		Command: []string{"echo", "OK"},
		Expect:  "OK",
		Timeout: 5 * time.Second,
	}
	result := runSingleFleetProbe(probe)
	if result.Duration < 0 {
		t.Fatalf("expected non-negative duration, got %s", result.Duration)
	}
}

func TestRunFleetProbes_AllGreen(t *testing.T) {
	old := fleetExecCommandContext
	fleetExecCommandContext = mockExecCustom("OK Ready Running Healthy active ok Up healthy")
	defer func() { fleetExecCommandContext = old }()

	probes := buildFleetProbes(defaultTestConfig())
	results := runFleetProbes(probes)
	if len(results) != 11 {
		t.Fatalf("expected 11 results, got %d", len(results))
	}
	green, _, _ := countFleetResults(results)
	if green != 11 {
		t.Fatalf("expected all 11 GREEN, got %d", green)
	}
}

func TestRunFleetProbes_AllGreen_Local(t *testing.T) {
	old := fleetExecCommandContext
	fleetExecCommandContext = mockExecCustom("Ready Running Healthy active ok Up healthy")
	defer func() { fleetExecCommandContext = old }()

	cfg := defaultTestConfig()
	cfg.LocalMode = true
	probes := buildFleetProbes(cfg)
	results := runFleetProbes(probes)
	if len(results) != 9 {
		t.Fatalf("expected 9 results in local mode, got %d", len(results))
	}
	green, _, _ := countFleetResults(results)
	if green != 9 {
		t.Fatalf("expected all 9 GREEN in local mode, got %d", green)
	}
}

func TestRunFleetProbes_MixedResults(t *testing.T) {
	callCount := 0
	old := fleetExecCommandContext
	fleetExecCommandContext = func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		callCount++
		if callCount == 1 {
			return []byte("OK"), nil
		}
		if callCount == 2 {
			return nil, fmt.Errorf("timeout")
		}
		return []byte("degraded"), nil
	}
	defer func() { fleetExecCommandContext = old }()

	probes := buildFleetProbes(defaultTestConfig())
	results := runFleetProbes(probes)
	green, yellow, red := countFleetResults(results)
	if green != 1 {
		t.Errorf("expected 1 GREEN, got %d", green)
	}
	if red != 1 {
		t.Errorf("expected 1 RED, got %d", red)
	}
	if yellow != 9 {
		t.Errorf("expected 9 YELLOW, got %d", yellow)
	}
}

func TestCountFleetResults(t *testing.T) {
	results := []FleetProbeResult{
		{Status: FleetGreen},
		{Status: FleetGreen},
		{Status: FleetYellow},
		{Status: FleetRed},
	}
	green, yellow, red := countFleetResults(results)
	if green != 2 || yellow != 1 || red != 1 {
		t.Fatalf("expected 2/1/1, got %d/%d/%d", green, yellow, red)
	}
}

func TestCountFleetResults_Empty(t *testing.T) {
	green, yellow, red := countFleetResults(nil)
	if green != 0 || yellow != 0 || red != 0 {
		t.Fatalf("expected 0/0/0, got %d/%d/%d", green, yellow, red)
	}
}

func TestWriteFleetJSON(t *testing.T) {
	results := []FleetProbeResult{
		{Name: "test-a", Status: FleetGreen, Output: "OK", Duration: 50 * time.Millisecond},
		{Name: "test-b", Status: FleetRed, Error: "fail", Duration: 100 * time.Millisecond},
	}
	var buf bytes.Buffer
	cmd := doctorFleetCmd
	cmd.SetOut(&buf)
	err := writeFleetJSON(cmd, results)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 NDJSON lines, got %d", len(lines))
	}

	var event struct {
		Event  string `json:"event"`
		Probe  string `json:"probe"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &event); err != nil {
		t.Fatalf("failed to parse first NDJSON line: %v", err)
	}
	if event.Event != "fleet_probe" {
		t.Errorf("expected event=fleet_probe, got %s", event.Event)
	}
	if event.Probe != "test-a" {
		t.Errorf("expected probe=test-a, got %s", event.Probe)
	}
	if event.Status != "GREEN" {
		t.Errorf("expected status=GREEN, got %s", event.Status)
	}
}

func TestPrintFleetTable(t *testing.T) {
	results := []FleetProbeResult{
		{Name: "SSH connectivity", Status: FleetGreen, Output: "OK"},
		{Name: "K3s nodes", Status: FleetRed, Error: "timeout"},
	}
	var buf bytes.Buffer
	cmd := doctorFleetCmd
	cmd.SetOut(&buf)
	printFleetTable(cmd, results)
	output := buf.String()
	if !strings.Contains(output, "SSH connectivity") {
		t.Error("expected table to contain probe name")
	}
	if !strings.Contains(output, "GREEN") {
		t.Error("expected table to contain GREEN")
	}
	if !strings.Contains(output, "RED") {
		t.Error("expected table to contain RED")
	}
}

func TestPrintFleetTable_TruncatesLongDetail(t *testing.T) {
	longOutput := strings.Repeat("x", 100)
	results := []FleetProbeResult{
		{Name: "long-output", Status: FleetGreen, Output: longOutput},
	}
	var buf bytes.Buffer
	cmd := doctorFleetCmd
	cmd.SetOut(&buf)
	printFleetTable(cmd, results)
	output := buf.String()
	if strings.Contains(output, longOutput) {
		t.Error("expected long output to be truncated")
	}
	if !strings.Contains(output, "...") {
		t.Error("expected truncated output to end with ...")
	}
}

func TestFleetConfigFromEnv_Defaults(t *testing.T) {
	doctorFleetFlags.sshTarget = ""
	t.Setenv("FLEET_SSH_TARGET", "")
	t.Setenv("FLEET_ENGRAM_HEALTHZ_URL", "")
	t.Setenv("FLEET_ENGRAM_TUNNEL_URL", "")
	t.Setenv("FLEET_DASHBOARD_URL", "")

	cfg := fleetConfigFromEnv()
	if cfg.SSHTarget != "wsl1-travel" {
		t.Errorf("expected default SSH target wsl1-travel, got %s", cfg.SSHTarget)
	}
	if cfg.EngramHealthzURL != "http://127.0.0.1:8280/healthz" {
		t.Errorf("unexpected EngramHealthzURL: %s", cfg.EngramHealthzURL)
	}
}

func TestFleetConfigFromEnv_OverrideViaEnv(t *testing.T) {
	doctorFleetFlags.sshTarget = ""
	t.Setenv("FLEET_SSH_TARGET", "custom-alias")

	cfg := fleetConfigFromEnv()
	if cfg.SSHTarget != "custom-alias" {
		t.Errorf("expected custom-alias, got %s", cfg.SSHTarget)
	}
}

func TestFleetConfigFromEnv_FlagOverridesEnv(t *testing.T) {
	t.Setenv("FLEET_SSH_TARGET", "env-alias")
	old := doctorFleetFlags.sshTarget
	doctorFleetFlags.sshTarget = "flag-alias"
	defer func() { doctorFleetFlags.sshTarget = old }()

	cfg := fleetConfigFromEnv()
	if cfg.SSHTarget != "flag-alias" {
		t.Errorf("expected flag-alias, got %s", cfg.SSHTarget)
	}
}

func TestEnvOrDefault(t *testing.T) {
	t.Setenv("TEST_FLEET_KEY", "custom-val")
	if v := envOrDefault("TEST_FLEET_KEY", "fallback"); v != "custom-val" {
		t.Errorf("expected custom-val, got %s", v)
	}
	t.Setenv("TEST_FLEET_KEY", "")
	if v := envOrDefault("TEST_FLEET_KEY", "fallback"); v != "fallback" {
		t.Errorf("expected fallback, got %s", v)
	}
}

func TestDoctorFleetCommandRegistered(t *testing.T) {
	names := []string{}
	for _, cmd := range doctorCmd.Commands() {
		names = append(names, cmd.Name())
	}
	if !containsString(names, "fleet") {
		t.Fatalf("doctor fleet command not registered; got %v", names)
	}
}

func TestFleetProbeStatus_Values(t *testing.T) {
	if FleetGreen != "GREEN" {
		t.Error("FleetGreen should be GREEN")
	}
	if FleetYellow != "YELLOW" {
		t.Error("FleetYellow should be YELLOW")
	}
	if FleetRed != "RED" {
		t.Error("FleetRed should be RED")
	}
}

func TestBuildFleetProbes_Names_Remote(t *testing.T) {
	probes := buildFleetProbes(defaultTestConfig())
	expected := []string{
		"SSH connectivity",
		"K3s nodes",
		"K3s pods (cicd)",
		"ArgoCD apps",
		"Engram systemd",
		"Engram healthz",
		"Fleet Agent systemd",
		"GitLab CE",
		"ArgoCD Docker",
		"Dashboard",
		"Engram tunnel",
	}
	if len(probes) != len(expected) {
		t.Fatalf("probe count mismatch: got %d, want %d", len(probes), len(expected))
	}
	for i, p := range probes {
		if p.Name != expected[i] {
			t.Errorf("probe[%d] name=%q, want %q", i, p.Name, expected[i])
		}
	}
}

func TestBuildFleetProbes_Names_Local(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.LocalMode = true
	probes := buildFleetProbes(cfg)
	expected := []string{
		"K3s nodes",
		"K3s pods (cicd)",
		"ArgoCD apps",
		"Engram systemd",
		"Engram healthz",
		"Fleet Agent systemd",
		"GitLab CE",
		"ArgoCD Docker",
		"Dashboard",
	}
	if len(probes) != len(expected) {
		t.Fatalf("probe count mismatch: got %d, want %d", len(probes), len(expected))
	}
	for i, p := range probes {
		if p.Name != expected[i] {
			t.Errorf("probe[%d] name=%q, want %q", i, p.Name, expected[i])
		}
	}
}

func TestBuildFleetProbes_RemoteUsesRunx(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.LocalMode = false
	probes := buildFleetProbes(cfg)
	for _, p := range probes {
		if p.Local {
			continue
		}
		if p.Command[0] != "runx" {
			t.Errorf("remote probe %q should use runx, got %s", p.Name, p.Command[0])
		}
	}
}

func TestPrintFleetTable_MultilineOutputFlattened(t *testing.T) {
	results := []FleetProbeResult{
		{Name: "multiline", Status: FleetGreen, Output: "line1\nline2\nline3"},
	}
	var buf bytes.Buffer
	cmd := doctorFleetCmd
	cmd.SetOut(&buf)
	printFleetTable(cmd, results)
	output := buf.String()
	if strings.Contains(output, "\nline2") {
		t.Error("expected newlines in output to be replaced")
	}
}
