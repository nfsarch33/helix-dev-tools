package ansiblevalidator_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/ansiblevalidator"
)

func TestValidateInventoryYAML_ValidStructure(t *testing.T) {
	inv := `---
all:
  children:
    fleet_linux:
      hosts:
        linux-host-1:
          ansible_host: 192.0.2.1
          ansible_port: 2233
          ansible_user: testuser
        linux-host-2:
          ansible_host: 192.0.2.2
          ansible_port: 2233
          ansible_user: testuser
    fleet_windows:
      hosts:
        windows-host-1:
          ansible_host: 192.0.2.3
          ansible_port: 22
          ansible_shell_type: powershell
    wsl_fleet:
      hosts:
        linux-host-1:
          ansible_host: 192.0.2.1
        linux-host-2:
          ansible_host: 192.0.2.2
`
	result := ansiblevalidator.ValidateInventoryYAML([]byte(inv))
	if !result.Valid {
		t.Fatalf("expected valid inventory, got errors: %v", result.Errors)
	}
}

func TestValidateInventoryYAML_MissingRequiredGroups(t *testing.T) {
	inv := `---
all:
  children:
    fleet_linux:
      hosts:
        linux-host-1:
          ansible_host: 192.0.2.1
`
	result := ansiblevalidator.ValidateInventoryYAML([]byte(inv))
	if result.Valid {
		t.Fatal("expected invalid inventory when required groups missing")
	}

	found := false
	for _, e := range result.Errors {
		if contains(e, "fleet_windows") || contains(e, "wsl_fleet") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error about missing fleet_windows or wsl_fleet, got: %v", result.Errors)
	}
}

func TestValidateInventoryYAML_InvalidYAML(t *testing.T) {
	result := ansiblevalidator.ValidateInventoryYAML([]byte("not: [valid: yaml: {"))
	if result.Valid {
		t.Fatal("expected invalid result for malformed YAML")
	}
}

func TestValidateInventoryYAML_HostMissingAnsibleHost(t *testing.T) {
	inv := `---
all:
  children:
    fleet_linux:
      hosts:
        linux-host-1:
          ansible_port: 2233
    fleet_windows:
      hosts:
        windows-host-1:
          ansible_host: 192.0.2.3
    wsl_fleet:
      hosts:
        linux-host-1:
          ansible_host: 192.0.2.1
`
	result := ansiblevalidator.ValidateInventoryYAML([]byte(inv))
	if result.Valid {
		t.Fatal("expected invalid: fleet_linux.linux-host-1 missing ansible_host")
	}
}

func TestValidatePlaybookYAML_ValidPlaybook(t *testing.T) {
	pb := `---
- name: Test playbook
  hosts: wsl_fleet
  gather_facts: false
  tasks:
    - name: Ensure directory exists
      ansible.builtin.file:
        path: /tmp/test
        state: directory
`
	result := ansiblevalidator.ValidatePlaybookYAML([]byte(pb))
	if !result.Valid {
		t.Fatalf("expected valid playbook, got errors: %v", result.Errors)
	}
}

func TestValidatePlaybookYAML_MissingHosts(t *testing.T) {
	pb := `---
- name: Bad playbook
  tasks:
    - name: Do nothing
      ansible.builtin.debug:
        msg: hello
`
	result := ansiblevalidator.ValidatePlaybookYAML([]byte(pb))
	if result.Valid {
		t.Fatal("expected invalid playbook without hosts field")
	}
}

func TestValidatePlaybookYAML_MissingTasks(t *testing.T) {
	pb := `---
- name: No tasks playbook
  hosts: all
`
	result := ansiblevalidator.ValidatePlaybookYAML([]byte(pb))
	if result.Valid {
		t.Fatal("expected invalid playbook without tasks")
	}
}

func TestValidatePlaybookYAML_TaskMissingName(t *testing.T) {
	pb := `---
- name: Unnamed task playbook
  hosts: wsl_fleet
  tasks:
    - ansible.builtin.debug:
        msg: hello
`
	result := ansiblevalidator.ValidatePlaybookYAML([]byte(pb))
	if result.Valid {
		t.Fatal("expected invalid when task has no name")
	}
}

func TestValidatePlaybookFile_FileNotFound(t *testing.T) {
	result := ansiblevalidator.ValidatePlaybookFile("/nonexistent/path.yml")
	if result.Valid {
		t.Fatal("expected invalid for nonexistent file")
	}
}

func TestValidatePlaybookFile_RealFile(t *testing.T) {
	dir := t.TempDir()
	pb := filepath.Join(dir, "test.yml")
	content := `---
- name: Valid play
  hosts: all
  tasks:
    - name: Ping
      ansible.builtin.ping:
`
	if err := os.WriteFile(pb, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result := ansiblevalidator.ValidatePlaybookFile(pb)
	if !result.Valid {
		t.Fatalf("expected valid, got: %v", result.Errors)
	}
}

func TestValidateInventoryRequiredGroups_CustomGroups(t *testing.T) {
	inv := `---
all:
  children:
    fleet_linux:
      hosts:
        linux-host-1:
          ansible_host: 192.0.2.1
    fleet_windows:
      hosts:
        windows-host-1:
          ansible_host: 192.0.2.2
    wsl_fleet:
      hosts:
        linux-host-1:
          ansible_host: 192.0.2.1
`
	result := ansiblevalidator.ValidateInventoryWithGroups(
		[]byte(inv),
		[]string{"fleet_linux", "fleet_windows", "wsl_fleet", "custom_group"},
	)
	if result.Valid {
		t.Fatal("expected invalid when custom_group is missing")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchStr(s, substr)
}

func searchStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
