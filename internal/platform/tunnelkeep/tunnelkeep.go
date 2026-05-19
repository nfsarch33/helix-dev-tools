package tunnelkeep

import (
	"fmt"
	"net/http"
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
	return TunnelConfig{
		Name:     "mem0-oracle",
		LocalURL: "http://127.0.0.1:18888/healthz",
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
		<string>/Users/jason.lian/.local/bin/cursor-tools</string>
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
	<string>/Users/jason.lian/logs/runx/tunnel-keepalive.log</string>
	<key>StandardErrorPath</key>
	<string>/Users/jason.lian/logs/runx/tunnel-keepalive.log</string>
</dict>
</plist>`, cfg.Name, cfg.Name, int(cfg.Interval.Seconds()))
}
