package ansiblevalidator_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/ansiblevalidator"
)

func skipIfNoRunx(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("runx"); err != nil {
		t.Skip("runx not in PATH; skipping integration test")
	}
}

// Integration tests use runx alias names only (no IPs or hostnames).
// The actual alias-to-host mapping is resolved by runx at runtime
// from ~/.config/runx/config.yaml.

const (
	testControlAlias = "your-control-node"
	testLinuxTarget  = "your-linux-host"
	testLinux2Alias  = "your-linux-host-2"
)

func TestIntegration_SSHCanary_LinuxHost(t *testing.T) {
	skipIfNoRunx(t)
	if os.Getenv("CURSOR_TOOLS_INTEGRATION") != "1" {
		t.Skip("set CURSOR_TOOLS_INTEGRATION=1 to run SSH integration tests")
	}

	alias := os.Getenv("CURSOR_TOOLS_SSH_ALIAS")
	if alias == "" {
		t.Skip("set CURSOR_TOOLS_SSH_ALIAS to the runx alias to probe")
	}

	result := ansiblevalidator.SSHCanary(alias)
	if !result.Reachable {
		t.Fatalf("SSH canary failed for alias %s: %s", alias, result.Error)
	}
	if result.HostAlias != alias {
		t.Errorf("expected alias %s, got %s", alias, result.HostAlias)
	}
}

func TestIntegration_SSHCanary_SecondHost(t *testing.T) {
	skipIfNoRunx(t)
	if os.Getenv("CURSOR_TOOLS_INTEGRATION") != "1" {
		t.Skip("set CURSOR_TOOLS_INTEGRATION=1 to run SSH integration tests")
	}

	alias := os.Getenv("CURSOR_TOOLS_SSH_ALIAS_2")
	if alias == "" {
		t.Skip("set CURSOR_TOOLS_SSH_ALIAS_2 for second host probe")
	}

	result := ansiblevalidator.SSHCanary(alias)
	if !result.Reachable {
		t.Logf("second host SSH canary not reachable (may be known limitation): %s", result.Error)
	}
}

func TestIntegration_SSHCanary_UnknownHost(t *testing.T) {
	skipIfNoRunx(t)
	if os.Getenv("CURSOR_TOOLS_INTEGRATION") != "1" {
		t.Skip("set CURSOR_TOOLS_INTEGRATION=1 to run SSH integration tests")
	}

	result := ansiblevalidator.SSHCanary("nonexistent-host-alias")
	if result.Reachable {
		t.Fatal("expected nonexistent host to be unreachable")
	}
}

func TestIntegration_AnsiblePing(t *testing.T) {
	skipIfNoRunx(t)
	if os.Getenv("CURSOR_TOOLS_INTEGRATION") != "1" {
		t.Skip("set CURSOR_TOOLS_INTEGRATION=1 to run Ansible integration tests")
	}

	controlAlias := os.Getenv("CURSOR_TOOLS_CONTROL_ALIAS")
	targetHost := os.Getenv("CURSOR_TOOLS_PING_TARGET")
	if controlAlias == "" || targetHost == "" {
		t.Skip("set CURSOR_TOOLS_CONTROL_ALIAS and CURSOR_TOOLS_PING_TARGET")
	}

	result := ansiblevalidator.AnsiblePing(controlAlias, targetHost)
	if !result.Success {
		t.Fatalf("Ansible ping to %s via %s failed: %s", targetHost, controlAlias, result.Error)
	}
}

func TestIntegration_InventoryGroupMembership(t *testing.T) {
	skipIfNoRunx(t)
	if os.Getenv("CURSOR_TOOLS_INTEGRATION") != "1" {
		t.Skip("set CURSOR_TOOLS_INTEGRATION=1 to run integration tests")
	}

	controlAlias := os.Getenv("CURSOR_TOOLS_CONTROL_ALIAS")
	if controlAlias == "" {
		t.Skip("set CURSOR_TOOLS_CONTROL_ALIAS to the runx alias of the Ansible control node")
	}

	groups, err := ansiblevalidator.ListInventoryGroups(controlAlias)
	if err != nil {
		t.Fatalf("failed to list inventory groups: %v", err)
	}

	required := []string{"fleet_linux", "wsl_fleet"}
	for _, g := range required {
		found := false
		for _, actual := range groups {
			if strings.TrimSpace(actual) == g {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("required group %q not found in inventory groups: %v", g, groups)
		}
	}
}

func TestSSHCanaryResult_String(t *testing.T) {
	r := ansiblevalidator.SSHCanaryResult{
		HostAlias: "test-host",
		Reachable: true,
	}
	s := r.String()
	if !strings.Contains(s, "test-host") {
		t.Errorf("String() should contain host alias, got: %s", s)
	}
}
