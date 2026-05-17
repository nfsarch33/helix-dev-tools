package ansiblevalidator

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// SSHCanaryResult holds the outcome of an SSH reachability probe.
type SSHCanaryResult struct {
	HostAlias string
	Reachable bool
	Latency   time.Duration
	Error     string
}

// AnsiblePingResult holds the outcome of an Ansible ping probe.
type AnsiblePingResult struct {
	Host    string
	Success bool
	Output  string
	Error   string
}

// SSHCanary probes a host via `runx ssh exec --target <alias> -- echo ok`
// and returns whether the host is reachable.
func SSHCanary(alias string) SSHCanaryResult {
	start := time.Now()
	cmd := exec.Command("runx", "ssh", "exec", "--target", alias, "--", "echo", "canary-ok")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	elapsed := time.Since(start)

	if err != nil {
		return SSHCanaryResult{
			HostAlias: alias,
			Reachable: false,
			Latency:   elapsed,
			Error:     fmt.Sprintf("%v: %s", err, strings.TrimSpace(stderr.String())),
		}
	}

	out := strings.TrimSpace(stdout.String())
	if !strings.Contains(out, "canary-ok") {
		return SSHCanaryResult{
			HostAlias: alias,
			Reachable: false,
			Latency:   elapsed,
			Error:     fmt.Sprintf("unexpected output: %q", out),
		}
	}

	return SSHCanaryResult{
		HostAlias: alias,
		Reachable: true,
		Latency:   elapsed,
	}
}

// AnsiblePing runs an Ansible ping module against a target host via runx
// SSH exec to the control node. The controlAlias is the runx alias for
// the Ansible control node (e.g. "your-control-node").
func AnsiblePing(controlAlias, targetHost string) AnsiblePingResult {
	script := fmt.Sprintf(
		"source ~/.venv/fleet-ansible/bin/activate 2>/dev/null; "+
			"ansible -m ping %s -i /tmp/fleet.tailnet-direct.yml 2>&1 || "+
			"ansible -m ping %s 2>&1",
		targetHost, targetHost,
	)
	cmd := exec.Command("runx", "ssh", "exec", "--target", controlAlias, "--", "bash", "-c", script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	out := strings.TrimSpace(stdout.String())

	if err != nil && !strings.Contains(out, "SUCCESS") {
		return AnsiblePingResult{
			Host:    targetHost,
			Success: false,
			Output:  out,
			Error:   fmt.Sprintf("%v: %s", err, strings.TrimSpace(stderr.String())),
		}
	}

	success := strings.Contains(out, "SUCCESS") || strings.Contains(out, "pong")
	result := AnsiblePingResult{
		Host:    targetHost,
		Success: success,
		Output:  out,
	}
	if !success {
		result.Error = "ping did not return SUCCESS or pong"
	}
	return result
}

// ListInventoryGroups queries the Ansible control node for inventory
// group names from the generated inventory.
func ListInventoryGroups(controlAlias string) ([]string, error) {
	script := "source ~/.venv/fleet-ansible/bin/activate 2>/dev/null; " +
		"ansible-inventory --list -i /tmp/fleet.tailnet-direct.yml 2>/dev/null | " +
		"python3 -c 'import sys,json; inv=json.load(sys.stdin); print(chr(10).join(k for k in inv if k not in (\"_meta\",\"all\")))'"

	cmd := exec.Command("runx", "ssh", "exec", "--target", controlAlias, "--", "bash", "-c", script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("listing groups: %v: %s", err, strings.TrimSpace(stderr.String()))
	}

	var groups []string
	for _, line := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			groups = append(groups, line)
		}
	}
	return groups, nil
}

// String returns a human-readable summary of the SSH canary result.
func (r SSHCanaryResult) String() string {
	if r.Reachable {
		return fmt.Sprintf("%s: reachable (%s)", r.HostAlias, r.Latency.Round(time.Millisecond))
	}
	return fmt.Sprintf("%s: unreachable (%s)", r.HostAlias, r.Error)
}
