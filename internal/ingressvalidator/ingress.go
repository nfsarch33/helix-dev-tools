package ingressvalidator

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Ingress represents a Kubernetes Ingress resource.
type Ingress struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name        string            `yaml:"name"`
		Namespace   string            `yaml:"namespace"`
		Annotations map[string]string `yaml:"annotations"`
	} `yaml:"metadata"`
	Spec struct {
		IngressClassName string        `yaml:"ingressClassName"`
		TLS              []TLSConfig   `yaml:"tls"`
		Rules            []IngressRule `yaml:"rules"`
	} `yaml:"spec"`
}

// TLSConfig holds TLS configuration for an Ingress.
type TLSConfig struct {
	Hosts      []string `yaml:"hosts"`
	SecretName string   `yaml:"secretName"`
}

// IngressRule defines routing rules for an Ingress host.
type IngressRule struct {
	Host string `yaml:"host"`
	HTTP struct {
		Paths []PathRule `yaml:"paths"`
	} `yaml:"http"`
}

// PathRule defines a single path routing rule.
type PathRule struct {
	Path     string `yaml:"path"`
	PathType string `yaml:"pathType"`
	Backend  struct {
		Service struct {
			Name string `yaml:"name"`
			Port struct {
				Number int `yaml:"number"`
			} `yaml:"port"`
		} `yaml:"service"`
	} `yaml:"backend"`
}

// ParseIngress parses a Kubernetes Ingress manifest from YAML.
func ParseIngress(manifest []byte) (*Ingress, error) {
	var ing Ingress
	if err := yaml.Unmarshal(manifest, &ing); err != nil {
		return nil, fmt.Errorf("parsing ingress: %w", err)
	}
	if ing.Kind != "Ingress" {
		return nil, fmt.Errorf("expected kind Ingress, got %q", ing.Kind)
	}
	return &ing, nil
}

// ValidateTLSConfig checks that TLS entries have secret names and that all rule
// hosts are covered by a TLS entry.
func ValidateTLSConfig(ingress Ingress) error {
	tlsHosts := make(map[string]bool)
	for _, tls := range ingress.Spec.TLS {
		if tls.SecretName == "" {
			return fmt.Errorf("TLS entry for hosts %v missing secretName", tls.Hosts)
		}
		for _, h := range tls.Hosts {
			tlsHosts[h] = true
		}
	}

	var uncovered []string
	for _, rule := range ingress.Spec.Rules {
		if rule.Host != "" && !tlsHosts[rule.Host] {
			uncovered = append(uncovered, rule.Host)
		}
	}
	if len(uncovered) > 0 {
		return fmt.Errorf("hosts not covered by TLS: [%s]", strings.Join(uncovered, ", "))
	}
	return nil
}

// ValidateIngressClass checks the ingressClassName matches the expected value.
func ValidateIngressClass(ingress Ingress, expectedClass string) error {
	if ingress.Spec.IngressClassName != expectedClass {
		return fmt.Errorf("ingress class is %q, expected %q",
			ingress.Spec.IngressClassName, expectedClass)
	}
	return nil
}

// ValidatePathRouting checks for duplicate paths and missing backends.
func ValidatePathRouting(ingress Ingress) error {
	for _, rule := range ingress.Spec.Rules {
		seen := make(map[string]bool)
		for _, p := range rule.HTTP.Paths {
			key := rule.Host + p.Path
			if seen[key] {
				return fmt.Errorf("duplicate path %q for host %q", p.Path, rule.Host)
			}
			seen[key] = true

			if p.Backend.Service.Name == "" {
				return fmt.Errorf("path %q on host %q has no backend service name",
					p.Path, rule.Host)
			}
			if p.Backend.Service.Port.Number == 0 {
				return fmt.Errorf("path %q on host %q has no backend port",
					p.Path, rule.Host)
			}
		}
	}
	return nil
}

// ValidateCertManager checks that cert-manager annotations are present.
func ValidateCertManager(annotations map[string]string) error {
	if _, ok := annotations["cert-manager.io/cluster-issuer"]; !ok {
		if _, ok := annotations["cert-manager.io/issuer"]; !ok {
			return fmt.Errorf("missing cert-manager cluster-issuer or issuer annotation")
		}
	}
	return nil
}
