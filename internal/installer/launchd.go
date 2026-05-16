package installer

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// LaunchdInstaller manages macOS LaunchAgent plists.
type LaunchdInstaller struct {
	PlistDir string // defaults to ~/Library/LaunchAgents
	Exec     CommandExecutor
}

// NewLaunchdInstaller returns a LaunchdInstaller pointing at the user's
// LaunchAgents directory.
func NewLaunchdInstaller(exec CommandExecutor) *LaunchdInstaller {
	home, _ := os.UserHomeDir()
	return &LaunchdInstaller{
		PlistDir: filepath.Join(home, "Library", "LaunchAgents"),
		Exec:     exec,
	}
}

func (l *LaunchdInstaller) plistPath(name string) string {
	return filepath.Join(l.PlistDir, name+".plist")
}

func (l *LaunchdInstaller) guiDomain() (string, error) {
	out, err := l.Exec.Run("id", "-u")
	if err != nil {
		return "", fmt.Errorf("get uid: %w", err)
	}
	uid := strings.TrimSpace(string(out))
	return "gui/" + uid, nil
}

// Install generates a plist from config, writes it, and bootstraps
// via launchctl.
func (l *LaunchdInstaller) Install(name string, config DaemonConfig) error {
	plistXML, err := GeneratePlist(config)
	if err != nil {
		return fmt.Errorf("generate plist: %w", err)
	}

	if err := os.MkdirAll(l.PlistDir, 0o755); err != nil {
		return fmt.Errorf("create plist dir: %w", err)
	}

	path := l.plistPath(name)
	if err := os.WriteFile(path, plistXML, 0o644); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}

	domain, err := l.guiDomain()
	if err != nil {
		return err
	}

	if _, err := l.Exec.Run("launchctl", "bootstrap", domain, path); err != nil {
		return fmt.Errorf("launchctl bootstrap: %w", err)
	}

	return nil
}

// Uninstall bootouts the service and removes the plist file.
func (l *LaunchdInstaller) Uninstall(name string) error {
	domain, err := l.guiDomain()
	if err != nil {
		return err
	}

	target := domain + "/" + name
	_, _ = l.Exec.Run("launchctl", "bootout", target)

	path := l.plistPath(name)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove plist: %w", err)
	}
	return nil
}

// Status queries launchctl print to determine the daemon's runtime state.
func (l *LaunchdInstaller) Status(name string) (DaemonStatus, error) {
	st := DaemonStatus{Installed: l.IsInstalled(name)}
	if !st.Installed {
		return st, nil
	}

	domain, err := l.guiDomain()
	if err != nil {
		return st, err
	}

	target := domain + "/" + name
	out, err := l.Exec.Run("launchctl", "print", target)
	if err != nil {
		return st, nil
	}

	st.Running, st.PID, st.LastExit = parseLaunchctlPrint(string(out))
	return st, nil
}

// IsInstalled returns true when the plist file exists on disk.
func (l *LaunchdInstaller) IsInstalled(name string) bool {
	_, err := os.Stat(l.plistPath(name))
	return err == nil
}

// parseLaunchctlPrint extracts running state, PID, and last exit code
// from `launchctl print` output.
func parseLaunchctlPrint(output string) (running bool, pid int, lastExit int) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "pid = ") {
			val := strings.TrimPrefix(line, "pid = ")
			if p, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
				pid = p
				running = true
			}
		}
		if strings.HasPrefix(line, "last exit code = ") {
			val := strings.TrimPrefix(line, "last exit code = ")
			if c, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
				lastExit = c
			}
		}
	}
	return
}

// plist XML structures

type plistDict struct {
	XMLName xml.Name    `xml:"dict"`
	Items   []plistItem `xml:",any"`
}

type plistItem struct {
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

// GeneratePlist renders a launchd plist XML document from a DaemonConfig.
func GeneratePlist(cfg DaemonConfig) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	buf.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	buf.WriteString(`<plist version="1.0">` + "\n")
	buf.WriteString("<dict>\n")

	writeKey(&buf, "Label")
	writeString(&buf, cfg.Label)

	writeKey(&buf, "ProgramArguments")
	buf.WriteString("  <array>\n")
	buf.WriteString("    <string>" + xmlEscape(cfg.Binary) + "</string>\n")
	for _, arg := range cfg.Args {
		buf.WriteString("    <string>" + xmlEscape(arg) + "</string>\n")
	}
	buf.WriteString("  </array>\n")

	if cfg.WorkingDir != "" {
		writeKey(&buf, "WorkingDirectory")
		writeString(&buf, cfg.WorkingDir)
	}

	if cfg.LogPath != "" {
		writeKey(&buf, "StandardOutPath")
		writeString(&buf, cfg.LogPath)
		writeKey(&buf, "StandardErrorPath")
		writeString(&buf, cfg.LogPath)
	}

	if cfg.Interval > 0 {
		writeKey(&buf, "StartInterval")
		buf.WriteString(fmt.Sprintf("  <integer>%d</integer>\n", cfg.Interval))
	} else {
		writeKey(&buf, "KeepAlive")
		buf.WriteString("  <true/>\n")
		writeKey(&buf, "RunAtLoad")
		buf.WriteString("  <true/>\n")
	}

	if len(cfg.Environment) > 0 {
		writeKey(&buf, "EnvironmentVariables")
		buf.WriteString("  <dict>\n")
		for k, v := range cfg.Environment {
			buf.WriteString("    <key>" + xmlEscape(k) + "</key>\n")
			buf.WriteString("    <string>" + xmlEscape(v) + "</string>\n")
		}
		buf.WriteString("  </dict>\n")
	}

	buf.WriteString("</dict>\n")
	buf.WriteString("</plist>\n")
	return buf.Bytes(), nil
}

func writeKey(buf *bytes.Buffer, key string) {
	buf.WriteString("  <key>" + key + "</key>\n")
}

func writeString(buf *bytes.Buffer, val string) {
	buf.WriteString("  <string>" + xmlEscape(val) + "</string>\n")
}

func xmlEscape(s string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}
