package cli

import "testing"

func TestWorkspaceCommandRegistered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "workspace" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("workspace command not registered on rootCmd")
	}
}
