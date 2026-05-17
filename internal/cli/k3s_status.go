package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/cursor-tools/internal/k3svalidator"
)

var k3sKubeconfigPath string

var k3sCmd = &cobra.Command{
	Use:   "k3s",
	Short: "K3s cluster management commands",
}

var k3sStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check K3s cluster status including version compatibility and node readiness",
	RunE:  runK3sStatus,
}

func init() {
	k3sStatusCmd.Flags().StringVar(&k3sKubeconfigPath, "kubeconfig", "", "path to kubeconfig (defaults to kubectl default)")
	k3sCmd.AddCommand(k3sStatusCmd)
}

// K3sStatusReport is the JSON output of k3s status.
type K3sStatusReport struct {
	Nodes             []k3svalidator.K3sNode      `json:"nodes"`
	Version           *k3svalidator.K3sVersionInfo `json:"version,omitempty"`
	AllReady          bool                         `json:"all_ready"`
	VersionCompatible bool                         `json:"version_compatible"`
	Errors            []string                     `json:"errors,omitempty"`
}

func runK3sStatus(_ *cobra.Command, _ []string) error {
	report := K3sStatusReport{}
	var errs []string

	nodeOutput, err := kubectlGetNodes(k3sKubeconfigPath)
	if err != nil {
		return fmt.Errorf("cannot reach cluster: %w", err)
	}

	nodes, err := k3svalidator.ParseNodeStatus(string(nodeOutput))
	if err != nil {
		return fmt.Errorf("parse nodes: %w", err)
	}
	report.Nodes = nodes

	if err := k3svalidator.ValidateAllReady(nodes); err != nil {
		errs = append(errs, err.Error())
	} else {
		report.AllReady = true
	}

	if err := k3svalidator.CheckVersionCompatibility(nodes); err != nil {
		errs = append(errs, err.Error())
	} else {
		report.VersionCompatible = true
	}

	verOutput, err := getK3sVersion()
	if err == nil {
		ver, parseErr := k3svalidator.ParseK3sVersion(string(verOutput))
		if parseErr == nil {
			report.Version = ver
		}
	}

	report.Errors = errs
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func kubectlGetNodes(kubeconfig string) ([]byte, error) {
	args := []string{"get", "nodes"}
	if kubeconfig != "" {
		args = append([]string{"--kubeconfig", kubeconfig}, args...)
	}
	return exec.Command("kubectl", args...).Output()
}

func getK3sVersion() ([]byte, error) {
	return exec.Command("k3s", "--version").Output()
}
