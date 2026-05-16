package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// SystemdInstaller manages systemd user units.
type SystemdInstaller struct {
	UnitDir string // defaults to ~/.config/systemd/user/
	Exec    CommandExecutor
}

// NewSystemdInstaller returns a SystemdInstaller pointing at the user's
// systemd directory.
func NewSystemdInstaller(exec CommandExecutor) *SystemdInstaller {
	home, _ := os.UserHomeDir()
	return &SystemdInstaller{
		UnitDir: filepath.Join(home, ".config", "systemd", "user"),
		Exec:    exec,
	}
}

func (s *SystemdInstaller) unitPath(name string) string {
	svc := name
	if !strings.HasSuffix(svc, ".service") {
		svc += ".service"
	}
	return filepath.Join(s.UnitDir, svc)
}

func (s *SystemdInstaller) unitName(name string) string {
	if strings.HasSuffix(name, ".service") {
		return name
	}
	return name + ".service"
}

// Install generates a systemd unit file, writes it, and starts the
// service via systemctl --user.
func (s *SystemdInstaller) Install(name string, config DaemonConfig) error {
	unit := GenerateUnit(config)

	if err := os.MkdirAll(s.UnitDir, 0o755); err != nil {
		return fmt.Errorf("create unit dir: %w", err)
	}

	path := s.unitPath(name)
	if err := os.WriteFile(path, []byte(unit), 0o644); err != nil {
		return fmt.Errorf("write unit: %w", err)
	}

	if _, err := s.Exec.Run("systemctl", "--user", "daemon-reload"); err != nil {
		return fmt.Errorf("daemon-reload: %w", err)
	}

	unitName := s.unitName(name)
	if _, err := s.Exec.Run("systemctl", "--user", "enable", "--now", unitName); err != nil {
		return fmt.Errorf("enable service: %w", err)
	}

	return nil
}

// Uninstall stops, disables, and removes the unit file.
func (s *SystemdInstaller) Uninstall(name string) error {
	unitName := s.unitName(name)
	_, _ = s.Exec.Run("systemctl", "--user", "stop", unitName)
	_, _ = s.Exec.Run("systemctl", "--user", "disable", unitName)

	path := s.unitPath(name)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove unit: %w", err)
	}

	_, _ = s.Exec.Run("systemctl", "--user", "daemon-reload")
	return nil
}

// Status queries systemctl to determine the daemon's runtime state.
func (s *SystemdInstaller) Status(name string) (DaemonStatus, error) {
	st := DaemonStatus{Installed: s.IsInstalled(name)}
	if !st.Installed {
		return st, nil
	}

	unitName := s.unitName(name)
	out, err := s.Exec.Run("systemctl", "--user", "show",
		"--property=ActiveState,MainPID,ExecMainStatus", unitName)
	if err != nil {
		return st, nil
	}

	st.Running, st.PID, st.LastExit = parseSystemctlShow(string(out))
	return st, nil
}

// IsInstalled returns true when the unit file exists on disk.
func (s *SystemdInstaller) IsInstalled(name string) bool {
	_, err := os.Stat(s.unitPath(name))
	return err == nil
}

// parseSystemctlShow extracts state from `systemctl show --property=...` output.
func parseSystemctlShow(output string) (running bool, pid int, lastExit int) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ActiveState=") {
			val := strings.TrimPrefix(line, "ActiveState=")
			running = val == "active"
		}
		if strings.HasPrefix(line, "MainPID=") {
			val := strings.TrimPrefix(line, "MainPID=")
			if p, err := strconv.Atoi(val); err == nil {
				pid = p
			}
		}
		if strings.HasPrefix(line, "ExecMainStatus=") {
			val := strings.TrimPrefix(line, "ExecMainStatus=")
			if c, err := strconv.Atoi(val); err == nil {
				lastExit = c
			}
		}
	}
	return
}

// GenerateUnit renders a systemd unit file from a DaemonConfig.
func GenerateUnit(cfg DaemonConfig) string {
	var b strings.Builder

	b.WriteString("[Unit]\n")
	b.WriteString(fmt.Sprintf("Description=%s\n", cfg.Label))
	b.WriteString("\n")

	b.WriteString("[Service]\n")
	b.WriteString("Type=simple\n")

	execStart := cfg.Binary
	if len(cfg.Args) > 0 {
		execStart += " " + strings.Join(cfg.Args, " ")
	}
	b.WriteString(fmt.Sprintf("ExecStart=%s\n", execStart))

	if cfg.WorkingDir != "" {
		b.WriteString(fmt.Sprintf("WorkingDirectory=%s\n", cfg.WorkingDir))
	}

	if cfg.LogPath != "" {
		b.WriteString(fmt.Sprintf("StandardOutput=append:%s\n", cfg.LogPath))
		b.WriteString(fmt.Sprintf("StandardError=append:%s\n", cfg.LogPath))
	}

	for k, v := range cfg.Environment {
		b.WriteString(fmt.Sprintf("Environment=%s=%s\n", k, v))
	}

	b.WriteString("Restart=on-failure\n")
	b.WriteString("RestartSec=5\n")
	b.WriteString("\n")

	if cfg.Interval > 0 {
		b.WriteString("[Timer]\n")
		b.WriteString(fmt.Sprintf("OnUnitActiveSec=%ds\n", cfg.Interval))
		b.WriteString("Persistent=true\n")
		b.WriteString("\n")
	}

	b.WriteString("[Install]\n")
	b.WriteString("WantedBy=default.target\n")

	return b.String()
}
