package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- systemd unit generation ---

func TestGenerateUnit_LongRunning(t *testing.T) {
	cfg := DaemonConfig{
		Label:      "cursor-resource-probe",
		Binary:     "/usr/local/bin/cursor-tools",
		Args:       []string{"resource-probe", "--interval", "300"},
		WorkingDir: "/tmp",
		LogPath:    "/tmp/probe.log",
	}

	unit := GenerateUnit(cfg)
	mustContainSys(t, unit, "[Unit]")
	mustContainSys(t, unit, "Description=cursor-resource-probe")
	mustContainSys(t, unit, "[Service]")
	mustContainSys(t, unit, "Type=simple")
	mustContainSys(t, unit, "ExecStart=/usr/local/bin/cursor-tools resource-probe --interval 300")
	mustContainSys(t, unit, "WorkingDirectory=/tmp")
	mustContainSys(t, unit, "StandardOutput=append:/tmp/probe.log")
	mustContainSys(t, unit, "StandardError=append:/tmp/probe.log")
	mustContainSys(t, unit, "Restart=on-failure")
	mustContainSys(t, unit, "RestartSec=5")
	mustContainSys(t, unit, "[Install]")
	mustContainSys(t, unit, "WantedBy=default.target")
}

func TestGenerateUnit_Periodic(t *testing.T) {
	cfg := DaemonConfig{
		Label:    "cursor-fleet-health",
		Binary:   "/usr/local/bin/cursor-tools",
		Args:     []string{"health-check"},
		Interval: 300,
	}

	unit := GenerateUnit(cfg)
	mustContainSys(t, unit, "[Timer]")
	mustContainSys(t, unit, "OnUnitActiveSec=300s")
	mustContainSys(t, unit, "Persistent=true")
}

func TestGenerateUnit_Environment(t *testing.T) {
	cfg := DaemonConfig{
		Label:  "test-env",
		Binary: "/bin/echo",
		Environment: map[string]string{
			"FOO": "bar",
		},
	}

	unit := GenerateUnit(cfg)
	mustContainSys(t, unit, "Environment=FOO=bar")
}

func TestGenerateUnit_MinimalConfig(t *testing.T) {
	cfg := DaemonConfig{
		Label:  "minimal",
		Binary: "/bin/true",
	}

	unit := GenerateUnit(cfg)
	mustContainSys(t, unit, "Description=minimal")
	mustContainSys(t, unit, "ExecStart=/bin/true")

	if strings.Contains(unit, "WorkingDirectory=") {
		t.Error("minimal config should not have WorkingDirectory")
	}
	if strings.Contains(unit, "StandardOutput=") {
		t.Error("minimal config should not have StandardOutput")
	}
}

// --- install flow ---

func TestSystemdInstaller_Install(t *testing.T) {
	tmpDir := t.TempDir()
	exec := newMockExec()

	inst := &SystemdInstaller{UnitDir: tmpDir, Exec: exec}

	cfg := DaemonConfig{
		Label:  "test-svc",
		Binary: "/bin/test",
	}

	if err := inst.Install("test-svc", cfg); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(tmpDir, "test-svc.service")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("unit not written: %v", err)
	}

	data, _ := os.ReadFile(path)
	mustContainSys(t, string(data), "Description=test-svc")

	if !exec.called("systemctl --user daemon-reload") {
		t.Error("expected systemctl daemon-reload call")
	}
	if !exec.called("systemctl --user enable --now test-svc.service") {
		t.Error("expected systemctl enable call")
	}
}

func TestSystemdInstaller_Uninstall(t *testing.T) {
	tmpDir := t.TempDir()
	exec := newMockExec()

	unitPath := filepath.Join(tmpDir, "test-svc.service")
	os.WriteFile(unitPath, []byte("[Unit]"), 0o644)

	inst := &SystemdInstaller{UnitDir: tmpDir, Exec: exec}

	if err := inst.Uninstall("test-svc"); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(unitPath); !os.IsNotExist(err) {
		t.Error("unit file should be removed")
	}

	if !exec.called("systemctl --user stop test-svc.service") {
		t.Error("expected systemctl stop call")
	}
	if !exec.called("systemctl --user disable test-svc.service") {
		t.Error("expected systemctl disable call")
	}
}

func TestSystemdInstaller_Status_Running(t *testing.T) {
	tmpDir := t.TempDir()
	exec := newMockExec()
	exec.outputs["systemctl --user show --property=ActiveState,MainPID,ExecMainStatus test-svc.service"] = `ActiveState=active
MainPID=9876
ExecMainStatus=0`

	unitPath := filepath.Join(tmpDir, "test-svc.service")
	os.WriteFile(unitPath, []byte("[Unit]"), 0o644)

	inst := &SystemdInstaller{UnitDir: tmpDir, Exec: exec}

	st, err := inst.Status("test-svc")
	if err != nil {
		t.Fatal(err)
	}

	if !st.Installed {
		t.Error("expected Installed=true")
	}
	if !st.Running {
		t.Error("expected Running=true")
	}
	if st.PID != 9876 {
		t.Errorf("expected PID=9876, got %d", st.PID)
	}
	if st.LastExit != 0 {
		t.Errorf("expected LastExit=0, got %d", st.LastExit)
	}
}

func TestSystemdInstaller_Status_NotInstalled(t *testing.T) {
	tmpDir := t.TempDir()
	exec := newMockExec()

	inst := &SystemdInstaller{UnitDir: tmpDir, Exec: exec}

	st, err := inst.Status("missing-svc")
	if err != nil {
		t.Fatal(err)
	}

	if st.Installed {
		t.Error("expected Installed=false")
	}
}

func TestSystemdInstaller_IsInstalled(t *testing.T) {
	tmpDir := t.TempDir()
	inst := &SystemdInstaller{UnitDir: tmpDir}

	if inst.IsInstalled("missing") {
		t.Error("expected false for missing unit")
	}

	os.WriteFile(filepath.Join(tmpDir, "present.service"), []byte(""), 0o644)
	if !inst.IsInstalled("present") {
		t.Error("expected true for present unit")
	}
}

// --- parseSystemctlShow ---

func TestParseSystemctlShow_Active(t *testing.T) {
	output := `ActiveState=active
MainPID=1234
ExecMainStatus=0`
	running, pid, lastExit := parseSystemctlShow(output)
	if !running {
		t.Error("expected running=true")
	}
	if pid != 1234 {
		t.Errorf("expected pid=1234, got %d", pid)
	}
	if lastExit != 0 {
		t.Errorf("expected lastExit=0, got %d", lastExit)
	}
}

func TestParseSystemctlShow_Inactive(t *testing.T) {
	output := `ActiveState=inactive
MainPID=0
ExecMainStatus=137`
	running, pid, lastExit := parseSystemctlShow(output)
	if running {
		t.Error("expected running=false")
	}
	if pid != 0 {
		t.Errorf("expected pid=0, got %d", pid)
	}
	if lastExit != 137 {
		t.Errorf("expected lastExit=137, got %d", lastExit)
	}
}

func TestParseSystemctlShow_Empty(t *testing.T) {
	running, pid, lastExit := parseSystemctlShow("")
	if running || pid != 0 || lastExit != 0 {
		t.Error("expected all zero values for empty output")
	}
}

// --- platform detection ---

func TestNewSystemdInstaller_DefaultDir(t *testing.T) {
	exec := newMockExec()
	inst := NewSystemdInstaller(exec)
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".config", "systemd", "user")
	if inst.UnitDir != expected {
		t.Errorf("expected UnitDir=%s, got %s", expected, inst.UnitDir)
	}
}

// --- helpers ---

func mustContainSys(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("expected output to contain %q, got:\n%s", needle, haystack)
	}
}
