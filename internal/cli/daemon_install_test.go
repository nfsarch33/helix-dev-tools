package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/installer"
)

// --- mock installer ---

type mockInstaller struct {
	installed map[string]installer.DaemonConfig
	statuses  map[string]installer.DaemonStatus
	installFn func(name string, cfg installer.DaemonConfig) error
}

func newMockInstaller() *mockInstaller {
	return &mockInstaller{
		installed: make(map[string]installer.DaemonConfig),
		statuses:  make(map[string]installer.DaemonStatus),
	}
}

func (m *mockInstaller) Install(name string, cfg installer.DaemonConfig) error {
	if m.installFn != nil {
		return m.installFn(name, cfg)
	}
	m.installed[name] = cfg
	return nil
}

func (m *mockInstaller) Uninstall(name string) error {
	delete(m.installed, name)
	return nil
}

func (m *mockInstaller) Status(name string) (installer.DaemonStatus, error) {
	if st, ok := m.statuses[name]; ok {
		return st, nil
	}
	return installer.DaemonStatus{}, nil
}

func (m *mockInstaller) IsInstalled(name string) bool {
	_, ok := m.installed[name]
	return ok
}

// --- subcommand existence ---

func TestDaemonInstallCmd_Exists(t *testing.T) {
	if daemonInstallCmd == nil {
		t.Fatal("daemonInstallCmd should be defined")
	}
	if daemonInstallCmd.Use != "install <name>" {
		t.Errorf("unexpected Use: %s", daemonInstallCmd.Use)
	}
}

func TestDaemonUninstallCmd_Exists(t *testing.T) {
	if daemonUninstallCmd == nil {
		t.Fatal("daemonUninstallCmd should be defined")
	}
}

func TestDaemonStatusCmd_Exists(t *testing.T) {
	if daemonStatusCmd == nil {
		t.Fatal("daemonStatusCmd should be defined")
	}
}

func TestDaemonCmd_HasSubcommands(t *testing.T) {
	found := make(map[string]bool)
	for _, sub := range daemonCmd.Commands() {
		found[sub.Name()] = true
	}
	for _, want := range []string{"install", "uninstall", "status"} {
		if !found[want] {
			t.Errorf("daemon command missing subcommand %q", want)
		}
	}
}

// --- install ---

func TestRunDaemonInstall_Known(t *testing.T) {
	mock := newMockInstaller()
	orig := newInstallerFunc
	defer func() { newInstallerFunc = orig }()
	newInstallerFunc = func(goos string) (installer.Installer, error) {
		return mock, nil
	}

	var buf bytes.Buffer
	err := runDaemonInstall(&buf, "cursor-resource-probe", "darwin")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(buf.String(), "[OK]") {
		t.Errorf("expected OK output, got: %s", buf.String())
	}

	if len(mock.installed) != 1 {
		t.Errorf("expected 1 installed, got %d", len(mock.installed))
	}
}

func TestRunDaemonInstall_Unknown(t *testing.T) {
	var buf bytes.Buffer
	err := runDaemonInstall(&buf, "nonexistent", "darwin")
	if err == nil {
		t.Fatal("expected error for unknown daemon")
	}
	if !strings.Contains(err.Error(), "unknown daemon") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunDaemonInstall_UnsupportedPlatform(t *testing.T) {
	orig := newInstallerFunc
	defer func() { newInstallerFunc = orig }()
	newInstallerFunc = newInstallerDefault

	var buf bytes.Buffer
	err := runDaemonInstall(&buf, "cursor-resource-probe", "windows")
	if err == nil {
		t.Fatal("expected error for windows")
	}
	if !strings.Contains(err.Error(), "unsupported platform") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunDaemonInstall_InstallerError(t *testing.T) {
	mock := newMockInstaller()
	mock.installFn = func(name string, cfg installer.DaemonConfig) error {
		return fmt.Errorf("permission denied")
	}
	orig := newInstallerFunc
	defer func() { newInstallerFunc = orig }()
	newInstallerFunc = func(goos string) (installer.Installer, error) {
		return mock, nil
	}

	var buf bytes.Buffer
	err := runDaemonInstall(&buf, "cursor-resource-probe", "darwin")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- uninstall ---

func TestRunDaemonUninstall_Known(t *testing.T) {
	mock := newMockInstaller()
	mock.installed["com.user.cursor-resource-probe"] = installer.DaemonConfig{}
	orig := newInstallerFunc
	defer func() { newInstallerFunc = orig }()
	newInstallerFunc = func(goos string) (installer.Installer, error) {
		return mock, nil
	}

	var buf bytes.Buffer
	err := runDaemonUninstall(&buf, "cursor-resource-probe", "darwin")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "[OK]") {
		t.Errorf("expected OK output, got: %s", buf.String())
	}
}

func TestRunDaemonUninstall_Unknown(t *testing.T) {
	var buf bytes.Buffer
	err := runDaemonUninstall(&buf, "nonexistent", "darwin")
	if err == nil {
		t.Fatal("expected error for unknown daemon")
	}
}

// --- status ---

func TestRunDaemonStatus_All(t *testing.T) {
	mock := newMockInstaller()
	mock.statuses["com.user.cursor-resource-probe"] = installer.DaemonStatus{
		Installed: true, Running: true, PID: 123,
	}
	mock.statuses["com.user.cursor-fleet-health"] = installer.DaemonStatus{
		Installed: true, Running: false, LastExit: 1,
	}
	orig := newInstallerFunc
	defer func() { newInstallerFunc = orig }()
	newInstallerFunc = func(goos string) (installer.Installer, error) {
		return mock, nil
	}

	var buf bytes.Buffer
	err := runDaemonStatus(&buf, "", "darwin")
	if err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "cursor-resource-probe") {
		t.Error("expected resource-probe in output")
	}
	if !strings.Contains(out, "cursor-fleet-health") {
		t.Error("expected fleet-health in output")
	}
}

func TestRunDaemonStatus_SingleDaemon(t *testing.T) {
	mock := newMockInstaller()
	mock.statuses["com.user.cursor-resource-probe"] = installer.DaemonStatus{
		Installed: true, Running: true, PID: 456,
	}
	orig := newInstallerFunc
	defer func() { newInstallerFunc = orig }()
	newInstallerFunc = func(goos string) (installer.Installer, error) {
		return mock, nil
	}

	var buf bytes.Buffer
	err := runDaemonStatus(&buf, "cursor-resource-probe", "darwin")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "running") {
		t.Errorf("expected running status, got: %s", buf.String())
	}
}

func TestRunDaemonStatus_UnknownDaemon(t *testing.T) {
	mock := newMockInstaller()
	orig := newInstallerFunc
	defer func() { newInstallerFunc = orig }()
	newInstallerFunc = func(goos string) (installer.Installer, error) {
		return mock, nil
	}

	var buf bytes.Buffer
	err := runDaemonStatus(&buf, "nonexistent", "darwin")
	if err == nil {
		t.Fatal("expected error for unknown daemon")
	}
}

// --- knownDaemons registry ---

func TestKnownDaemons_Contains(t *testing.T) {
	expected := []string{"cursor-resource-probe", "cursor-fleet-health"}
	for _, name := range expected {
		if _, ok := knownDaemons[name]; !ok {
			t.Errorf("missing known daemon: %s", name)
		}
	}
}

func TestKnownDaemonNames_NonEmpty(t *testing.T) {
	names := knownDaemonNames()
	if names == "" {
		t.Error("knownDaemonNames returned empty string")
	}
	if !strings.Contains(names, "cursor-resource-probe") {
		t.Error("missing cursor-resource-probe")
	}
}

// --- platformLabel ---

func TestPlatformLabel_Darwin(t *testing.T) {
	label := platformLabel("cursor-resource-probe")
	// Can't control runtime.GOOS in test, so just verify it returns non-empty
	if label == "" {
		t.Error("expected non-empty label")
	}
}
