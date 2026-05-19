package tunnelkeep

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type TunnelConfig struct {
	Name     string
	LocalURL string
	Interval time.Duration
	Timeout  time.Duration
}

type ProbeResult struct {
	Healthy   bool      `json:"healthy"`
	Latency   int64     `json:"latency_ms"`
	Error     string    `json:"error,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

func DefaultMem0Config() TunnelConfig {
	localURL := os.Getenv("MEM0_HEALTH_URL")
	if localURL == "" {
		localURL = "http://localhost:8888/healthz"
	}
	return TunnelConfig{
		Name:     "mem0-oracle",
		LocalURL: localURL,
		Interval: 5 * time.Minute,
		Timeout:  10 * time.Second,
	}
}

func Probe(cfg TunnelConfig) ProbeResult {
	start := time.Now()
	client := &http.Client{Timeout: cfg.Timeout}

	resp, err := client.Get(cfg.LocalURL)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return ProbeResult{
			Healthy:   false,
			Latency:   latency,
			Error:     err.Error(),
			Timestamp: time.Now(),
		}
	}
	defer resp.Body.Close()

	return ProbeResult{
		Healthy:   resp.StatusCode == http.StatusOK,
		Latency:   latency,
		Timestamp: time.Now(),
	}
}

func ShouldRestart(results []ProbeResult, threshold int) bool {
	if len(results) < threshold {
		return false
	}
	consecutive := 0
	for i := len(results) - 1; i >= 0 && i >= len(results)-threshold; i-- {
		if !results[i].Healthy {
			consecutive++
		}
	}
	return consecutive >= threshold
}

func GenerateLaunchdPlist(cfg TunnelConfig) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.user.tunnel-keepalive-%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>tunnel</string>
		<string>keepalive</string>
		<string>--name</string>
		<string>%s</string>
	</array>
	<key>StartInterval</key>
	<integer>%d</integer>
	<key>RunAtLoad</key>
	<true/>
	<key>StandardOutPath</key>
	<string>%s</string>
	<key>StandardErrorPath</key>
	<string>%s</string>
</dict>
</plist>`, cfg.Name, binaryPath(), cfg.Name, int(cfg.Interval.Seconds()), logPath(), logPath())
}

func binaryPath() string {
	if p := os.Getenv("HELIX_TOOLS_BIN"); p != "" {
		return p
	}
	return filepath.Join(os.Getenv("HOME"), ".local", "bin", "cursor-tools")
}

func logPath() string {
	if p := os.Getenv("HELIX_TOOLS_LOG_DIR"); p != "" {
		return filepath.Join(p, "tunnel-keepalive.log")
	}
	return filepath.Join(os.Getenv("HOME"), "logs", "runx", "tunnel-keepalive.log")
}
