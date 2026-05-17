package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/helix-dev-tools/internal/fleetk8s"
)

var fleetKubeconfigPath string

var fleetCmd = &cobra.Command{
	Use:   "fleet",
	Short: "Fleet management commands",
}

var fleetK8sStatusCmd = &cobra.Command{
	Use:   "k8s-status",
	Short: "Report K8s cluster node, pod, and GPU status as JSON",
	Long: `Queries kubectl for nodes and pods, parses GPU capacity from
nvidia.com/gpu resources, and returns a structured JSON report.
Requires kubectl on PATH and a valid kubeconfig.`,
	RunE: runFleetK8sStatus,
}

func init() {
	fleetK8sStatusCmd.Flags().StringVar(&fleetKubeconfigPath, "kubeconfig", "", "path to kubeconfig (defaults to kubectl default)")
	fleetCmd.AddCommand(fleetK8sStatusCmd)
}

func runFleetK8sStatus(_ *cobra.Command, _ []string) error {
	runner := &fleetk8s.DefaultRunner{Kubeconfig: fleetKubeconfigPath}
	collector := fleetk8s.NewCollector(runner)
	status, err := collector.Collect()
	if err != nil {
		return fmt.Errorf("fleet k8s-status: %w", err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(status)
}
