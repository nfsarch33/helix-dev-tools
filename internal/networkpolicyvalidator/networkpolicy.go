package networkpolicyvalidator

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// NetworkPolicy represents a Kubernetes NetworkPolicy resource.
type NetworkPolicy struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
	Spec struct {
		PodSelector struct {
			MatchLabels map[string]string `yaml:"matchLabels"`
		} `yaml:"podSelector"`
		PolicyTypes []string      `yaml:"policyTypes"`
		Ingress     []IngressRule `yaml:"ingress"`
		Egress      []EgressRule  `yaml:"egress"`
	} `yaml:"spec"`
}

// IngressRule defines an ingress rule in a NetworkPolicy.
type IngressRule struct {
	From  []Peer     `yaml:"from"`
	Ports []PortRule `yaml:"ports"`
}

// EgressRule defines an egress rule in a NetworkPolicy.
type EgressRule struct {
	To    []Peer     `yaml:"to"`
	Ports []PortRule `yaml:"ports"`
}

// Peer represents a network peer selector.
type Peer struct {
	PodSelector *struct {
		MatchLabels map[string]string `yaml:"matchLabels"`
	} `yaml:"podSelector"`
	IPBlock *struct {
		CIDR string `yaml:"cidr"`
	} `yaml:"ipBlock"`
}

// PortRule defines a port and protocol for a network rule.
type PortRule struct {
	Protocol string `yaml:"protocol"`
	Port     int    `yaml:"port"`
}

// ParseNetworkPolicy parses a Kubernetes NetworkPolicy manifest from YAML.
func ParseNetworkPolicy(manifest []byte) (*NetworkPolicy, error) {
	var np NetworkPolicy
	if err := yaml.Unmarshal(manifest, &np); err != nil {
		return nil, fmt.Errorf("parsing network policy: %w", err)
	}
	if np.Kind != "NetworkPolicy" {
		return nil, fmt.Errorf("expected kind NetworkPolicy, got %q", np.Kind)
	}
	return &np, nil
}

// ValidateNamespaceIsolation verifies that a namespace has a default-deny policy
// before any allow rules take effect.
func ValidateNamespaceIsolation(policies []NetworkPolicy, namespace string) error {
	hasDefaultDeny := false
	for _, p := range policies {
		if p.Metadata.Namespace != namespace {
			continue
		}
		if len(p.Spec.PodSelector.MatchLabels) == 0 &&
			len(p.Spec.Ingress) == 0 && len(p.Spec.Egress) == 0 {
			hasDefaultDeny = true
			break
		}
	}
	if !hasDefaultDeny {
		return fmt.Errorf("namespace %q missing default-deny network policy", namespace)
	}
	return nil
}

// ValidatePodToPodPolicy checks whether a NetworkPolicy allows traffic from
// pods matching sourceLabels to pods matching destLabels.
func ValidatePodToPodPolicy(policy NetworkPolicy, sourceLabels, destLabels map[string]string) error {
	if !labelsMatch(policy.Spec.PodSelector.MatchLabels, destLabels) {
		return fmt.Errorf("policy %q does not target destination pods", policy.Metadata.Name)
	}

	for _, rule := range policy.Spec.Ingress {
		for _, from := range rule.From {
			if from.PodSelector != nil && labelsMatch(from.PodSelector.MatchLabels, sourceLabels) {
				return nil
			}
		}
	}

	return fmt.Errorf("no ingress rule allows traffic from source labels %v", sourceLabels)
}

// ValidateEgressRules verifies that a NetworkPolicy has egress rules permitting
// traffic to at least one of the allowed CIDRs.
func ValidateEgressRules(policy NetworkPolicy, allowedCIDRs []string) error {
	if len(policy.Spec.Egress) == 0 {
		return fmt.Errorf("policy %q has no egress rules defined", policy.Metadata.Name)
	}

	allowed := make(map[string]bool, len(allowedCIDRs))
	for _, c := range allowedCIDRs {
		allowed[c] = true
	}

	for _, rule := range policy.Spec.Egress {
		for _, to := range rule.To {
			if to.IPBlock != nil && allowed[to.IPBlock.CIDR] {
				return nil
			}
		}
	}

	return fmt.Errorf("no egress rule matches allowed CIDRs [%s]", strings.Join(allowedCIDRs, ", "))
}

// ValidateIngressRules verifies that a NetworkPolicy only exposes allowed ports.
func ValidateIngressRules(policy NetworkPolicy, allowedPorts []int) error {
	portSet := make(map[int]bool, len(allowedPorts))
	for _, p := range allowedPorts {
		portSet[p] = true
	}

	var disallowed []string
	for _, rule := range policy.Spec.Ingress {
		for _, port := range rule.Ports {
			if !portSet[port.Port] {
				disallowed = append(disallowed, fmt.Sprintf("%d", port.Port))
			}
		}
	}

	if len(disallowed) > 0 {
		return fmt.Errorf("ingress exposes disallowed ports: [%s]; allowed: %v",
			strings.Join(disallowed, ", "), allowedPorts)
	}

	return nil
}

func labelsMatch(selector, target map[string]string) bool {
	for k, v := range selector {
		if target[k] != v {
			return false
		}
	}
	return true
}
