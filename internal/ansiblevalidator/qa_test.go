package ansiblevalidator_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nfsarch33/cursor-tools/internal/ansiblevalidator"
)

func TestQA_ValidateRealMem0OutboxPlaybook(t *testing.T) {
	ansibleWin := os.Getenv("ANSIBLE_WIN_PATH")
	if ansibleWin == "" {
		home, _ := os.UserHomeDir()
		ansibleWin = filepath.Join(home, "ansible-win")
	}

	pb := filepath.Join(ansibleWin, "ansible", "playbooks", "mem0-outbox-wsl.yml")
	if _, err := os.Stat(pb); os.IsNotExist(err) {
		t.Skipf("ansible-win not found at %s", pb)
	}

	result := ansiblevalidator.ValidatePlaybookFile(pb)
	if !result.Valid {
		t.Fatalf("mem0-outbox-wsl.yml failed validation: %v", result.Errors)
	}
}

func TestQA_PlaybookIdempotencyChecks(t *testing.T) {
	tests := []struct {
		name     string
		playbook string
		valid    bool
	}{
		{
			name: "idempotent file module",
			playbook: `---
- name: Idempotent play
  hosts: wsl_fleet
  gather_facts: false
  tasks:
    - name: Ensure config dir
      ansible.builtin.file:
        path: /tmp/testdir
        state: directory
        mode: "0755"
    - name: Copy config
      ansible.builtin.copy:
        dest: /tmp/testdir/config.yml
        content: "key: value\n"
        mode: "0644"
`,
			valid: true,
		},
		{
			name: "play with command module",
			playbook: `---
- name: Command play
  hosts: all
  tasks:
    - name: Run arbitrary command
      ansible.builtin.command: echo hello
`,
			valid: true,
		},
		{
			name: "drift check playbook pattern",
			playbook: `---
- name: Drift check
  hosts: fleet_linux
  gather_facts: true
  tasks:
    - name: Verify SSH config
      ansible.builtin.stat:
        path: /etc/ssh/sshd_config
      register: sshd_config
    - name: Assert SSH config exists
      ansible.builtin.assert:
        that: sshd_config.stat.exists
        fail_msg: "sshd_config missing on {{ inventory_hostname }}"
`,
			valid: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ansiblevalidator.ValidatePlaybookYAML([]byte(tc.playbook))
			if result.Valid != tc.valid {
				t.Errorf("expected valid=%v, got valid=%v: %v", tc.valid, result.Valid, result.Errors)
			}
		})
	}
}

func TestQA_InventoryAcceptance(t *testing.T) {
	tests := []struct {
		name      string
		inventory string
		valid     bool
	}{
		{
			name: "minimal fleet inventory",
			inventory: `---
all:
  children:
    fleet_linux:
      hosts:
        linux-host-1:
          ansible_host: 192.0.2.1
          ansible_port: 2233
          ansible_user: testuser
          ansible_ssh_private_key_file: ~/.ssh/your-key
    fleet_windows:
      hosts:
        windows-host-1:
          ansible_host: 192.0.2.2
          ansible_port: 22
          ansible_shell_type: powershell
          ansible_user: testuser
    wsl_fleet:
      hosts:
        linux-host-1:
          ansible_host: 192.0.2.1
`,
			valid: true,
		},
		{
			name: "inventory with vars section",
			inventory: `---
all:
  vars:
    ansible_ssh_common_args: "-o StrictHostKeyChecking=accept-new"
  children:
    fleet_linux:
      hosts:
        linux-host-1:
          ansible_host: 192.0.2.1
      vars:
        ansible_port: 2233
    fleet_windows:
      hosts:
        windows-host-1:
          ansible_host: 192.0.2.2
    wsl_fleet:
      hosts:
        linux-host-1:
          ansible_host: 192.0.2.1
`,
			valid: true,
		},
		{
			name: "empty children",
			inventory: `---
all:
  children: {}
`,
			valid: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ansiblevalidator.ValidateInventoryYAML([]byte(tc.inventory))
			if result.Valid != tc.valid {
				t.Errorf("expected valid=%v, got valid=%v errors: %v",
					tc.valid, result.Valid, result.Errors)
			}
		})
	}
}

func TestQA_DriftCheckPlaybookPattern(t *testing.T) {
	pb := `---
- name: Fleet drift check
  hosts: fleet_linux
  gather_facts: true
  tasks:
    - name: Check systemd is running
      ansible.builtin.command: systemctl is-system-running
      register: systemd_state
      changed_when: false
      failed_when: false
    - name: Assert systemd healthy
      ansible.builtin.assert:
        that:
          - systemd_state.rc == 0
          - "'running' in systemd_state.stdout or 'degraded' in systemd_state.stdout"
        fail_msg: "systemd not healthy: {{ systemd_state.stdout }}"
    - name: Check SSH service
      ansible.builtin.systemd:
        name: ssh
      register: ssh_service
    - name: Assert SSH active
      ansible.builtin.assert:
        that: ssh_service.status.ActiveState == "active"
`
	result := ansiblevalidator.ValidatePlaybookYAML([]byte(pb))
	if !result.Valid {
		t.Fatalf("drift check playbook should be valid: %v", result.Errors)
	}
}

func TestQA_FleetOnboardingPlaybookPattern(t *testing.T) {
	pb := `---
- name: Fleet onboarding
  hosts: wsl_fleet
  become: true
  gather_facts: true
  tasks:
    - name: Update apt cache
      ansible.builtin.apt:
        update_cache: true
        cache_valid_time: 3600
    - name: Install base packages
      ansible.builtin.apt:
        name:
          - curl
          - git
          - jq
          - python3-venv
        state: present
    - name: Ensure automation user exists
      ansible.builtin.user:
        name: automation
        shell: /bin/bash
        state: present
    - name: Ensure .ssh directory
      ansible.builtin.file:
        path: /home/automation/.ssh
        state: directory
        owner: automation
        group: automation
        mode: "0700"
`
	result := ansiblevalidator.ValidatePlaybookYAML([]byte(pb))
	if !result.Valid {
		t.Fatalf("fleet onboarding playbook should be valid: %v", result.Errors)
	}
}
