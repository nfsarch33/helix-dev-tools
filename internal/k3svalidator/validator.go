package k3svalidator

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// K3sNode holds parsed node information from kubectl get nodes output.
type K3sNode struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Roles   string `json:"roles"`
	Age     string `json:"age"`
	Version string `json:"version"`
}

// K3sVersionInfo holds parsed k3s version output.
type K3sVersionInfo struct {
	K3sVersion  string `json:"k3s_version"`
	KubeVersion string `json:"kube_version"`
	GoVersion   string `json:"go_version"`
}

// ParseNodeStatus parses the textual output of `kubectl get nodes`.
func ParseNodeStatus(output string) ([]K3sNode, error) {
	output = strings.TrimSpace(output)
	if output == "" {
		return nil, fmt.Errorf("empty node status output")
	}

	lines := strings.Split(output, "\n")
	if len(lines) < 1 {
		return nil, fmt.Errorf("no header line in node output")
	}

	nodes := make([]K3sNode, 0, len(lines)-1)
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		roles := fields[2]
		if roles == "<none>" {
			roles = "worker"
		}
		nodes = append(nodes, K3sNode{
			Name:    fields[0],
			Status:  fields[1],
			Roles:   roles,
			Age:     fields[3],
			Version: fields[4],
		})
	}
	return nodes, nil
}

// ValidateAllReady checks that every node reports Ready status.
func ValidateAllReady(nodes []K3sNode) error {
	if len(nodes) == 0 {
		return fmt.Errorf("no nodes found in cluster")
	}
	var notReady []string
	for _, n := range nodes {
		if n.Status != "Ready" {
			notReady = append(notReady, n.Name)
		}
	}
	if len(notReady) > 0 {
		return fmt.Errorf("nodes not ready: %s", strings.Join(notReady, ", "))
	}
	return nil
}

var k3sVersionRe = regexp.MustCompile(`k3s version (v\d+\.\d+\.\d+\+k3s\d+)`)
var goVersionRe = regexp.MustCompile(`go version (go[\d.]+)`)

// ParseK3sVersion parses the output of `k3s --version`.
func ParseK3sVersion(output string) (*K3sVersionInfo, error) {
	output = strings.TrimSpace(output)
	if output == "" {
		return nil, fmt.Errorf("empty version output")
	}

	matches := k3sVersionRe.FindStringSubmatch(output)
	if matches == nil {
		return nil, fmt.Errorf("cannot parse k3s version from output")
	}

	info := &K3sVersionInfo{
		K3sVersion: matches[1],
	}

	parts := strings.SplitN(matches[1], "+", 2)
	info.KubeVersion = strings.TrimPrefix(parts[0], "v")

	goMatches := goVersionRe.FindStringSubmatch(output)
	if goMatches != nil {
		info.GoVersion = goMatches[1]
	}

	return info, nil
}

// CheckVersionCompatibility ensures all nodes are within 1 minor version of each other.
func CheckVersionCompatibility(nodes []K3sNode) error {
	if len(nodes) < 2 {
		return nil
	}

	type semver struct {
		major, minor int
		node         string
	}

	versions := make([]semver, 0, len(nodes))
	for _, n := range nodes {
		m, mi, err := parseMinorVersion(n.Version)
		if err != nil {
			return fmt.Errorf("node %s: %w", n.Name, err)
		}
		versions = append(versions, semver{major: m, minor: mi, node: n.Name})
	}

	for i := 1; i < len(versions); i++ {
		if versions[i].major != versions[0].major {
			return fmt.Errorf("major version mismatch: %s vs %s", versions[0].node, versions[i].node)
		}
		diff := versions[0].minor - versions[i].minor
		if diff < 0 {
			diff = -diff
		}
		if diff > 1 {
			return fmt.Errorf("version skew too large (>1 minor): %s has minor %d, %s has minor %d",
				versions[0].node, versions[0].minor, versions[i].node, versions[i].minor)
		}
	}
	return nil
}

func parseMinorVersion(version string) (int, int, error) {
	version = strings.TrimPrefix(version, "v")
	parts := strings.SplitN(version, "+", 2)
	semParts := strings.Split(parts[0], ".")
	if len(semParts) < 2 {
		return 0, 0, fmt.Errorf("invalid version format: %s", version)
	}
	major, err := strconv.Atoi(semParts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid major version: %s", semParts[0])
	}
	minor, err := strconv.Atoi(semParts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid minor version: %s", semParts[1])
	}
	return major, minor, nil
}

// kubeconfig YAML structures for validation
type kubeconfigFile struct {
	APIVersion string            `yaml:"apiVersion"`
	Kind       string            `yaml:"kind"`
	Clusters   []kubeconfigEntry `yaml:"clusters"`
}

type kubeconfigEntry struct {
	Cluster kubeconfigCluster `yaml:"cluster"`
	Name    string            `yaml:"name"`
}

type kubeconfigCluster struct {
	Server                string `yaml:"server"`
	CertificateAuthority  string `yaml:"certificate-authority"`
	CertificateAuthData   string `yaml:"certificate-authority-data"`
}

// ValidateKubeconfig checks that a kubeconfig byte slice is structurally valid.
func ValidateKubeconfig(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("kubeconfig is empty")
	}

	var cfg kubeconfigFile
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("invalid kubeconfig YAML: %w", err)
	}

	if len(cfg.Clusters) == 0 {
		return fmt.Errorf("kubeconfig has no clusters defined")
	}

	for _, c := range cfg.Clusters {
		if c.Cluster.Server == "" {
			return fmt.Errorf("cluster %q has no server URL", c.Name)
		}
	}

	return nil
}
