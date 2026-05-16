package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- mock executor ---

type mockExec struct {
	calls   [][]string
	outputs map[string]string
	errs    map[string]error
}

func newMockExec() *mockExec {
	return &mockExec{
		outputs: make(map[string]string),
		errs:    make(map[string]error),
	}
}

func (m *mockExec) Run(name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	m.calls = append(m.calls, append([]string{name}, args...))
	if err, ok := m.errs[key]; ok {
		return nil, err
	}
	return []byte(m.outputs[key]), nil
}

func (m *mockExec) called(substr string) bool {
	for _, c := range m.calls {
		if strings.Contains(strings.Join(c, " "), substr) {
			return true
		}
	}
	return false
}

// --- plist generation ---

func TestGeneratePlist_LongRunning(t *testing.T) {
	cfg := DaemonConfig{
		Label:      "com.user.cursor-resource-probe",
		Binary:     "/usr/local/bin/cursor-tools",
		Args:       []string{"resource-probe", "--interval", "300"},
		WorkingDir: "/tmp",
		LogPath:    "/tmp/probe.log",
	}

	data, err := GeneratePlist(cfg)
	if err != nil {
		t.Fatal(err)
	}

	xml := string(data)
	mustContain(t, xml, "<string>com.user.cursor-resource-probe</string>")
	mustContain(t, xml, "<string>/usr/local/bin/cursor-tools</string>")
	mustContain(t, xml, "<string>resource-probe</string>")
	mustContain(t, xml, "<string>--interval</string>")
	mustContain(t, xml, "<string>300</string>")
	mustContain(t, xml, "<key>WorkingDirectory</key>")
	mustContain(t, xml, "<string>/tmp</string>")
	mustContain(t, xml, "<key>StandardOutPath</key>")
	mustContain(t, xml, "<key>StandardErrorPath</key>")
	mustContain(t, xml, "<key>KeepAlive</key>")
	mustContain(t, xml, "<true/>")
	mustContain(t, xml, "<key>RunAtLoad</key>")
	mustContain(t, xml, `<?xml version="1.0"`)
	mustContain(t, xml, `<!DOCTYPE plist`)
}

func TestGeneratePlist_Periodic(t *testing.T) {
	cfg := DaemonConfig{
		Label:    "com.user.cursor-fleet-health",
		Binary:   "/usr/local/bin/cursor-tools",
		Args:     []string{"health-check"},
		Interval: 300,
	}

	data, err := GeneratePlist(cfg)
	if err != nil {
		t.Fatal(err)
	}

	xml := string(data)
	mustContain(t, xml, "<key>StartInterval</key>")
	mustContain(t, xml, "<integer>300</integer>")

	if strings.Contains(xml, "<key>KeepAlive</key>") {
		t.Error("periodic daemon should not have KeepAlive")
	}
}

func TestGeneratePlist_EnvironmentVariables(t *testing.T) {
	cfg := DaemonConfig{
		Label:  "com.user.test-env",
		Binary: "/bin/echo",
		Environment: map[string]string{
			"FOO": "bar",
		},
	}

	data, err := GeneratePlist(cfg)
	if err != nil {
		t.Fatal(err)
	}

	xml := string(data)
	mustContain(t, xml, "<key>EnvironmentVariables</key>")
	mustContain(t, xml, "<key>FOO</key>")
	mustContain(t, xml, "<string>bar</string>")
}

func TestGeneratePlist_XMLEscape(t *testing.T) {
	cfg := DaemonConfig{
		Label:  "test",
		Binary: "/bin/echo",
		Args:   []string{"<hello>&world"},
	}

	data, err := GeneratePlist(cfg)
	if err != nil {
		t.Fatal(err)
	}

	xml := string(data)
	mustContain(t, xml, "&lt;hello&gt;&amp;world")
}

// --- install flow ---

func TestLaunchdInstaller_Install(t *testing.T) {
	tmpDir := t.TempDir()
	exec := newMockExec()
	exec.outputs["id -u"] = "501\n"

	inst := &LaunchdInstaller{PlistDir: tmpDir, Exec: exec}

	cfg := DaemonConfig{
		Label:  "com.user.test",
		Binary: "/bin/test",
	}

	if err := inst.Install("com.user.test", cfg); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(tmpDir, "com.user.test.plist")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("plist not written: %v", err)
	}

	data, _ := os.ReadFile(path)
	mustContain(t, string(data), "<string>com.user.test</string>")

	if !exec.called("launchctl bootstrap gui/501") {
		t.Error("expected launchctl bootstrap call")
	}
}

func TestLaunchdInstaller_Uninstall(t *testing.T) {
	tmpDir := t.TempDir()
	exec := newMockExec()
	exec.outputs["id -u"] = "501\n"

	plistPath := filepath.Join(tmpDir, "com.user.test.plist")
	os.WriteFile(plistPath, []byte("<plist/>"), 0o644)

	inst := &LaunchdInstaller{PlistDir: tmpDir, Exec: exec}

	if err := inst.Uninstall("com.user.test"); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(plistPath); !os.IsNotExist(err) {
		t.Error("plist should be removed")
	}

	if !exec.called("launchctl bootout gui/501/com.user.test") {
		t.Error("expected launchctl bootout call")
	}
}

func TestLaunchdInstaller_Status_Installed(t *testing.T) {
	tmpDir := t.TempDir()
	exec := newMockExec()
	exec.outputs["id -u"] = "501\n"
	exec.outputs["launchctl print gui/501/com.user.test"] = `com.user.test = {
	pid = 12345
	last exit code = 0
}`

	plistPath := filepath.Join(tmpDir, "com.user.test.plist")
	os.WriteFile(plistPath, []byte("<plist/>"), 0o644)

	inst := &LaunchdInstaller{PlistDir: tmpDir, Exec: exec}

	st, err := inst.Status("com.user.test")
	if err != nil {
		t.Fatal(err)
	}

	if !st.Installed {
		t.Error("expected Installed=true")
	}
	if !st.Running {
		t.Error("expected Running=true")
	}
	if st.PID != 12345 {
		t.Errorf("expected PID=12345, got %d", st.PID)
	}
	if st.LastExit != 0 {
		t.Errorf("expected LastExit=0, got %d", st.LastExit)
	}
}

func TestLaunchdInstaller_Status_NotInstalled(t *testing.T) {
	tmpDir := t.TempDir()
	exec := newMockExec()

	inst := &LaunchdInstaller{PlistDir: tmpDir, Exec: exec}

	st, err := inst.Status("com.user.missing")
	if err != nil {
		t.Fatal(err)
	}

	if st.Installed {
		t.Error("expected Installed=false")
	}
	if st.Running {
		t.Error("expected Running=false")
	}
}

func TestLaunchdInstaller_IsInstalled(t *testing.T) {
	tmpDir := t.TempDir()
	inst := &LaunchdInstaller{PlistDir: tmpDir}

	if inst.IsInstalled("missing") {
		t.Error("expected false for missing plist")
	}

	os.WriteFile(filepath.Join(tmpDir, "present.plist"), []byte(""), 0o644)
	if !inst.IsInstalled("present") {
		t.Error("expected true for present plist")
	}
}

// --- parseLaunchctlPrint ---

func TestParseLaunchctlPrint_Running(t *testing.T) {
	output := `com.user.test = {
	active count = 1
	pid = 42
	last exit code = 0
	state = running
}`
	running, pid, lastExit := parseLaunchctlPrint(output)
	if !running {
		t.Error("expected running=true")
	}
	if pid != 42 {
		t.Errorf("expected pid=42, got %d", pid)
	}
	if lastExit != 0 {
		t.Errorf("expected lastExit=0, got %d", lastExit)
	}
}

func TestParseLaunchctlPrint_Stopped(t *testing.T) {
	output := `com.user.test = {
	active count = 0
	last exit code = 1
	state = not running
}`
	running, pid, lastExit := parseLaunchctlPrint(output)
	if running {
		t.Error("expected running=false")
	}
	if pid != 0 {
		t.Errorf("expected pid=0, got %d", pid)
	}
	if lastExit != 1 {
		t.Errorf("expected lastExit=1, got %d", lastExit)
	}
}

func TestParseLaunchctlPrint_Empty(t *testing.T) {
	running, pid, lastExit := parseLaunchctlPrint("")
	if running || pid != 0 || lastExit != 0 {
		t.Error("expected all zero values for empty output")
	}
}

// --- platform detection ---

func TestNewLaunchdInstaller_DefaultDir(t *testing.T) {
	exec := newMockExec()
	inst := NewLaunchdInstaller(exec)
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, "Library", "LaunchAgents")
	if inst.PlistDir != expected {
		t.Errorf("expected PlistDir=%s, got %s", expected, inst.PlistDir)
	}
}

// --- helpers ---

func mustContain(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("expected output to contain %q, got:\n%s", needle, haystack)
	}
}
