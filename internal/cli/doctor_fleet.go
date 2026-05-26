// runx-public-repo-gate: allow-file fleet_host_alias
// runx-public-repo-gate: allow-file network_topology
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type FleetProbeStatus string

const (
	FleetGreen  FleetProbeStatus = "GREEN"
	FleetYellow FleetProbeStatus = "YELLOW"
	FleetRed    FleetProbeStatus = "RED"
)

type FleetProbe struct {
	Name    string        `json:"name"`
	Command []string      `json:"-"`
	Expect  string        `json:"-"`
	Timeout time.Duration `json:"-"`
	Local   bool          `json:"-"`
}

type FleetProbeResult struct {
	Name     string           `json:"name"`
	Status   FleetProbeStatus `json:"status"`
	Output   string           `json:"output,omitempty"`
	Error    string           `json:"error,omitempty"`
	Duration time.Duration    `json:"duration_ms"`
}

type FleetConfig struct {
	SSHTarget        string
	EngramHealthzURL string
	EngramTunnelURL  string
	DashboardURL     string
}

var fleetExecCommandContext = execCommandContext

var doctorFleetFlags struct {
	sshTarget string
}

var doctorFleetCmd = &cobra.Command{
	Use:   "fleet",
	Short: "Probe fleet SSH, K3s, services, and tunnels",
	Long: "Run 11 fleet health probes covering SSH connectivity, K3s cluster,\n" +
		"systemd services, Docker containers, and local tunnels.\n" +
		"Uses runx ssh exec for remote probes and curl for local endpoints.",
	RunE: func(cmd *cobra.Command, _ []string) error {
		cfg := fleetConfigFromEnv()
		probes := buildFleetProbes(cfg)
		results := runFleetProbes(probes)

		if doctorOutputJSON {
			return writeFleetJSON(cmd, results)
		}
		printFleetTable(cmd, results)
		green, yellow, red := countFleetResults(results)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nFleet health: %d/%d GREEN, %d YELLOW, %d RED\n",
			green, len(results), yellow, red)
		if red > 0 {
			return fmt.Errorf("fleet health: %d probe(s) RED", red)
		}
		return nil
	},
}

func init() {
	doctorFleetCmd.Flags().StringVar(&doctorFleetFlags.sshTarget, "ssh-target", "",
		"SSH target alias for remote probes (default: FLEET_SSH_TARGET or wsl1-travel)")
	doctorCmd.AddCommand(doctorFleetCmd)
}

func fleetConfigFromEnv() FleetConfig {
	cfg := FleetConfig{
		SSHTarget:        envOrDefault("FLEET_SSH_TARGET", "wsl1-travel"),
		EngramHealthzURL: envOrDefault("FLEET_ENGRAM_HEALTHZ_URL", "http://127.0.0.1:8280/healthz"),
		EngramTunnelURL:  envOrDefault("FLEET_ENGRAM_TUNNEL_URL", "http://127.0.0.1:18888/healthz"),
		DashboardURL:     envOrDefault("FLEET_DASHBOARD_URL", "http://127.0.0.1:9095/api/health"),
	}
	if doctorFleetFlags.sshTarget != "" {
		cfg.SSHTarget = doctorFleetFlags.sshTarget
	}
	return cfg
}

func envOrDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func buildFleetProbes(cfg FleetConfig) []FleetProbe {
	t := cfg.SSHTarget
	return []FleetProbe{
		{
			Name:    "SSH connectivity",
			Command: []string{"runx", "ssh", "exec", "--target", t, "--raw", "echo OK"},
			Expect:  "OK",
			Timeout: 10 * time.Second,
		},
		{
			Name:    "K3s nodes",
			Command: []string{"runx", "ssh", "exec", "--target", t, "--raw", "sudo -n k3s kubectl get nodes --no-headers"},
			Expect:  "Ready",
			Timeout: 10 * time.Second,
		},
		{
			Name:    "K3s pods (cicd)",
			Command: []string{"runx", "ssh", "exec", "--target", t, "--raw", "sudo -n k3s kubectl -n cicd get pods --no-headers"},
			Expect:  "Running",
			Timeout: 10 * time.Second,
		},
		{
			Name:    "ArgoCD apps",
			Command: []string{"runx", "ssh", "exec", "--target", t, "--raw", "sudo -n k3s kubectl get applications -n argocd --no-headers"},
			Expect:  "Healthy",
			Timeout: 10 * time.Second,
		},
		{
			Name:    "Engram systemd",
			Command: []string{"runx", "ssh", "exec", "--target", t, "--raw", "systemctl is-active engram"},
			Expect:  "active",
			Timeout: 10 * time.Second,
		},
		{
			Name:    "Engram healthz",
			Command: []string{"runx", "ssh", "exec", "--target", t, "--raw", fmt.Sprintf("curl -sS --max-time 5 %s", cfg.EngramHealthzURL)},
			Expect:  "ok",
			Timeout: 10 * time.Second,
		},
		{
			Name:    "Fleet Agent systemd",
			Command: []string{"runx", "ssh", "exec", "--target", t, "--raw", "systemctl is-active fleet-agent"},
			Expect:  "active",
			Timeout: 10 * time.Second,
		},
		{
			Name:    "GitLab CE",
			Command: []string{"runx", "ssh", "exec", "--target", t, "--raw", "docker inspect --format '{{.State.Health.Status}}' gitlab-ce"},
			Expect:  "healthy",
			Timeout: 10 * time.Second,
		},
		{
			Name:    "ArgoCD Docker",
			Command: []string{"runx", "ssh", "exec", "--target", t, "--raw", "docker ps --filter name=argocd-server --format '{{.Status}}'"},
			Expect:  "Up",
			Timeout: 10 * time.Second,
		},
		{
			Name:    "Dashboard",
			Command: []string{"runx", "ssh", "exec", "--target", t, "--raw", fmt.Sprintf("curl -sS --max-time 5 %s", cfg.DashboardURL)},
			Expect:  "ok",
			Timeout: 10 * time.Second,
		},
		{
			Name:    "Engram tunnel",
			Command: []string{"curl", "-sS", "--max-time", "5", cfg.EngramTunnelURL},
			Expect:  "ok",
			Timeout: 10 * time.Second,
			Local:   true,
		},
	}
}

func runFleetProbes(probes []FleetProbe) []FleetProbeResult {
	results := make([]FleetProbeResult, 0, len(probes))
	for _, p := range probes {
		results = append(results, runSingleFleetProbe(p))
	}
	return results
}

func runSingleFleetProbe(p FleetProbe) FleetProbeResult {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), p.Timeout)
	defer cancel()

	out, err := fleetExecCommandContext(ctx, p.Command[0], p.Command[1:]...)
	elapsed := time.Since(start)
	trimmed := strings.TrimSpace(string(out))

	result := FleetProbeResult{
		Name:     p.Name,
		Output:   trimmed,
		Duration: elapsed.Truncate(time.Millisecond),
	}

	if err != nil {
		result.Status = FleetRed
		result.Error = err.Error()
		return result
	}

	if strings.Contains(trimmed, p.Expect) {
		result.Status = FleetGreen
	} else {
		result.Status = FleetYellow
	}
	return result
}

func execCommandContext(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

func printFleetTable(cmd *cobra.Command, results []FleetProbeResult) {
	w := cmd.OutOrStdout()
	_, _ = fmt.Fprintf(w, "%-25s %-8s %s\n", "PROBE", "STATUS", "DETAIL")
	_, _ = fmt.Fprintf(w, "%-25s %-8s %s\n", strings.Repeat("-", 25), strings.Repeat("-", 8), strings.Repeat("-", 40))
	for _, r := range results {
		detail := r.Output
		if r.Error != "" {
			detail = r.Error
		}
		if len(detail) > 60 {
			detail = detail[:57] + "..."
		}
		detail = strings.ReplaceAll(detail, "\n", " ")
		_, _ = fmt.Fprintf(w, "%-25s %-8s %s\n", r.Name, string(r.Status), detail)
	}
}

func writeFleetJSON(cmd *cobra.Command, results []FleetProbeResult) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	for _, r := range results {
		event := struct {
			Ts       string           `json:"ts"`
			Event    string           `json:"event"`
			Probe    string           `json:"probe"`
			Status   FleetProbeStatus `json:"status"`
			Output   string           `json:"output,omitempty"`
			Error    string           `json:"error,omitempty"`
			Duration int64            `json:"duration_ms"`
		}{
			Ts:       time.Now().Format(time.RFC3339),
			Event:    "fleet_probe",
			Probe:    r.Name,
			Status:   r.Status,
			Output:   r.Output,
			Error:    r.Error,
			Duration: r.Duration.Milliseconds(),
		}
		if err := enc.Encode(event); err != nil {
			return err
		}
	}
	return nil
}

func countFleetResults(results []FleetProbeResult) (green, yellow, red int) {
	for _, r := range results {
		switch r.Status {
		case FleetGreen:
			green++
		case FleetYellow:
			yellow++
		case FleetRed:
			red++
		}
	}
	return
}
