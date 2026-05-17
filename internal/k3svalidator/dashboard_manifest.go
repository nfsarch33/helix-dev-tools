package k3svalidator

import (
	"fmt"
	"net"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// DashboardManifest represents the expected K8s Dashboard deployment state.
type DashboardManifest struct {
	Namespace      string `json:"namespace"`
	ServiceType    string `json:"service_type"`
	NodePort       int    `json:"node_port,omitempty"`
	HasRBAC        bool   `json:"has_rbac"`
	ServiceAccount string `json:"service_account"`
}

// DashboardValidationResult holds the results of manifest validation.
type DashboardValidationResult struct {
	NamespaceExists   bool     `json:"namespace_exists"`
	DeploymentReady   bool     `json:"deployment_ready"`
	ServiceExists     bool     `json:"service_exists"`
	RBACConfigured    bool     `json:"rbac_configured"`
	NodePortAccessible bool    `json:"nodeport_accessible"`
	Errors            []string `json:"errors,omitempty"`
}

// RBACRule represents a Kubernetes RBAC rule for validation.
type RBACRule struct {
	APIGroups []string `yaml:"apiGroups" json:"api_groups"`
	Resources []string `yaml:"resources" json:"resources"`
	Verbs     []string `yaml:"verbs" json:"verbs"`
}

// ClusterRole represents a K8s ClusterRole for validation.
type ClusterRole struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Rules []RBACRule `yaml:"rules"`
}

// ServiceAccount represents a K8s ServiceAccount for validation.
type ServiceAccount struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
}

// ValidateDashboardRBAC checks that a ClusterRole YAML has appropriate scoping.
func ValidateDashboardRBAC(yamlData []byte) error {
	if len(yamlData) == 0 {
		return fmt.Errorf("empty RBAC manifest")
	}

	var role ClusterRole
	if err := yaml.Unmarshal(yamlData, &role); err != nil {
		return fmt.Errorf("invalid ClusterRole YAML: %w", err)
	}

	if role.Kind != "ClusterRole" && role.Kind != "Role" {
		return fmt.Errorf("expected ClusterRole or Role, got %q", role.Kind)
	}

	if role.Metadata.Name == "" {
		return fmt.Errorf("ClusterRole has no name")
	}

	for _, rule := range role.Rules {
		if containsWildcard(rule.Resources) && containsWildcard(rule.Verbs) {
			return fmt.Errorf("overly permissive rule: wildcard resources AND verbs in role %q", role.Metadata.Name)
		}
	}

	return nil
}

// ValidateServiceAccount checks that a ServiceAccount YAML is valid.
func ValidateServiceAccount(yamlData []byte) error {
	if len(yamlData) == 0 {
		return fmt.Errorf("empty ServiceAccount manifest")
	}

	var sa ServiceAccount
	if err := yaml.Unmarshal(yamlData, &sa); err != nil {
		return fmt.Errorf("invalid ServiceAccount YAML: %w", err)
	}

	if sa.Kind != "ServiceAccount" {
		return fmt.Errorf("expected ServiceAccount, got %q", sa.Kind)
	}

	if sa.Metadata.Name == "" {
		return fmt.Errorf("ServiceAccount has no name")
	}

	if sa.Metadata.Namespace == "" {
		return fmt.Errorf("ServiceAccount has no namespace")
	}

	return nil
}

// ValidateNodePortService checks that a service definition uses NodePort on the expected port.
func ValidateNodePortService(yamlData []byte, expectedPort int) error {
	if len(yamlData) == 0 {
		return fmt.Errorf("empty service manifest")
	}

	var svc struct {
		Kind string `yaml:"kind"`
		Spec struct {
			Type  string `yaml:"type"`
			Ports []struct {
				NodePort int `yaml:"nodePort"`
				Port     int `yaml:"port"`
			} `yaml:"ports"`
		} `yaml:"spec"`
	}

	if err := yaml.Unmarshal(yamlData, &svc); err != nil {
		return fmt.Errorf("invalid service YAML: %w", err)
	}

	if svc.Kind != "Service" {
		return fmt.Errorf("expected Service, got %q", svc.Kind)
	}

	if svc.Spec.Type != "NodePort" {
		return fmt.Errorf("service type is %q, expected NodePort", svc.Spec.Type)
	}

	if expectedPort > 0 {
		found := false
		for _, p := range svc.Spec.Ports {
			if p.NodePort == expectedPort {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("expected NodePort %d not found in service spec", expectedPort)
		}
	}

	return nil
}

// CheckPortAccessibility probes if a TCP port is reachable on a host.
func CheckPortAccessibility(host string, port int, timeout time.Duration) error {
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return fmt.Errorf("port %d not accessible on %s: %w", port, host, err)
	}
	conn.Close()
	return nil
}

func containsWildcard(items []string) bool {
	for _, item := range items {
		if strings.TrimSpace(item) == "*" {
			return true
		}
	}
	return false
}
