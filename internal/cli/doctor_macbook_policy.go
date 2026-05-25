// runx-public-repo-gate: allow-file fleet_host_alias,internal_service_id — EvoLoop client filters Mem0 capsules by the canonical evoloop-daemon source label and producer-machine name

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

type macbookPolicyState struct {
	TailscaleBinary   bool
	TailscaleApp      bool
	TailscaleLaunchD  bool
	TailscaledProcess bool
	ReplicaHealth     string
}

var doctorMacbookPolicyCmd = &cobra.Command{
	Use:   "macbook-policy",
	Short: "Verify this macbook stays off Tailscale and keeps only the local EvoLoop replica",
	RunE: func(cmd *cobra.Command, _ []string) error {
		state := gatherMacbookPolicyState()
		failures := evaluateMacbookPolicy(state)
		for _, failure := range failures {
			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "FAIL "+failure)
		}
		if len(failures) > 0 {
			return fmt.Errorf("macbook-policy failed: %d failure(s)", len(failures))
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "PASS macbook-policy")
		return nil
	},
}

func init() {
	doctorCmd.AddCommand(doctorMacbookPolicyCmd)
}

func evaluateMacbookPolicy(state macbookPolicyState) []string {
	var failures []string
	if state.TailscaleBinary {
		failures = append(failures, "tailscale binary is present")
	}
	if state.TailscaleApp {
		failures = append(failures, "Tailscale.app is installed")
	}
	if state.TailscaleLaunchD {
		failures = append(failures, "tailscaled launchd plist is installed")
	}
	if state.TailscaledProcess {
		failures = append(failures, "tailscaled process is running")
	}
	if state.ReplicaHealth != "healthy" {
		failures = append(failures, "evoloop-daemon-replica health is "+emptyAs(state.ReplicaHealth, "unknown"))
	}
	return failures
}

func gatherMacbookPolicyState() macbookPolicyState {
	return macbookPolicyState{
		TailscaleBinary:   commandExists("tailscale"),
		TailscaleApp:      pathExists("/Applications/Tailscale.app"),
		TailscaleLaunchD:  pathExists("/Library/LaunchDaemons/com.tailscale.tailscaled.plist"),
		TailscaledProcess: processExists("tailscaled"),
		ReplicaHealth:     dockerHealth("evoloop-daemon-replica"),
	}
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func processExists(name string) bool {
	cmd := exec.Command("pgrep", "-x", name)
	return cmd.Run() == nil
}

func dockerHealth(container string) string {
	out, err := exec.Command("docker", "inspect", container, "--format", "{{.State.Health.Status}}").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func emptyAs(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
