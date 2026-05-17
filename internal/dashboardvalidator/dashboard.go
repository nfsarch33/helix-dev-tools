package dashboardvalidator

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type deployment struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
	Spec struct {
		Template struct {
			Spec struct {
				Containers []container `yaml:"containers"`
			} `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

type container struct {
	Name  string `yaml:"name"`
	Image string `yaml:"image"`
	Resources struct {
		Limits   map[string]string `yaml:"limits"`
		Requests map[string]string `yaml:"requests"`
	} `yaml:"resources"`
	ReadinessProbe *probe `yaml:"readinessProbe"`
}

type probe struct {
	HTTPGet *struct {
		Path string `yaml:"path"`
		Port int    `yaml:"port"`
	} `yaml:"httpGet"`
}

type service struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
	Spec struct {
		Type  string `yaml:"type"`
		Ports []struct {
			Port       int `yaml:"port"`
			TargetPort int `yaml:"targetPort"`
			NodePort   int `yaml:"nodePort"`
		} `yaml:"ports"`
	} `yaml:"spec"`
}

// ParseDashboardVersion extracts the image tag from a dashboard Deployment manifest.
func ParseDashboardVersion(manifest []byte) (string, error) {
	var dep deployment
	if err := yaml.Unmarshal(manifest, &dep); err != nil {
		return "", fmt.Errorf("parsing deployment: %w", err)
	}

	for _, c := range dep.Spec.Template.Spec.Containers {
		if c.Image == "" {
			continue
		}
		parts := strings.SplitN(c.Image, ":", 2)
		if len(parts) == 2 {
			return parts[1], nil
		}
		return "", fmt.Errorf("image %q has no tag", c.Image)
	}

	return "", fmt.Errorf("no container image found in manifest")
}

// ValidateVersionCompatibility checks if a dashboard version is compatible with a K3s version.
// Legacy v2.x dashboard is incompatible with K3s >= v1.31 (requires v7.x / split architecture).
func ValidateVersionCompatibility(dashboardVersion, k3sVersion string) error {
	dv := strings.TrimPrefix(dashboardVersion, "v")

	if strings.HasPrefix(dv, "2.") || strings.HasPrefix(dv, "1.") && !isV7Component(dv) {
		k3sMajorMinor := extractK3sMinor(k3sVersion)
		if k3sMajorMinor >= 31 {
			return fmt.Errorf("dashboard v%s incompatible with K3s v1.%d: legacy dashboard v2.x requires K3s < v1.31; upgrade to v7.x split architecture", dv, k3sMajorMinor)
		}
	}

	return nil
}

// isV7Component returns true if the version looks like a v7.x split-arch component
// (dashboard-web 1.x.x or dashboard-api 1.x.x).
func isV7Component(version string) bool {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return false
	}
	major := parts[0]
	return major == "1"
}

func extractK3sMinor(version string) int {
	v := strings.TrimPrefix(version, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		return 0
	}
	minor := parts[1]
	minor = strings.Split(minor, "+")[0]
	minor = strings.Split(minor, "-")[0]
	var n int
	fmt.Sscanf(minor, "%d", &n)
	return n
}

// ValidateDashboardPod validates that a dashboard Deployment has resource limits and a readiness probe.
func ValidateDashboardPod(podYAML []byte) error {
	var dep deployment
	if err := yaml.Unmarshal(podYAML, &dep); err != nil {
		return fmt.Errorf("parsing deployment: %w", err)
	}

	var errs []string
	for _, c := range dep.Spec.Template.Spec.Containers {
		if len(c.Resources.Limits) == 0 {
			errs = append(errs, fmt.Sprintf("container %q missing resource limits", c.Name))
		}
		if c.ReadinessProbe == nil {
			errs = append(errs, fmt.Sprintf("container %q missing readiness probe", c.Name))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

// ValidateRBACConfig validates that multi-document RBAC YAML contains both a ServiceAccount
// and a ClusterRoleBinding.
func ValidateRBACConfig(rbacYAML []byte) error {
	docs := strings.Split(string(rbacYAML), "---")

	hasServiceAccount := false
	hasClusterRoleBinding := false

	for _, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}
		var obj struct {
			Kind string `yaml:"kind"`
		}
		if err := yaml.Unmarshal([]byte(doc), &obj); err != nil {
			continue
		}
		switch obj.Kind {
		case "ServiceAccount":
			hasServiceAccount = true
		case "ClusterRoleBinding":
			hasClusterRoleBinding = true
		}
	}

	var missing []string
	if !hasServiceAccount {
		missing = append(missing, "ServiceAccount")
	}
	if !hasClusterRoleBinding {
		missing = append(missing, "ClusterRoleBinding")
	}

	if len(missing) > 0 {
		return fmt.Errorf("RBAC config missing: %s", strings.Join(missing, ", "))
	}
	return nil
}

// ValidateNodePortAccess validates that a Service is type NodePort on the expected port
// within the valid Kubernetes NodePort range (30000-32767).
func ValidateNodePortAccess(serviceYAML []byte, expectedPort int) error {
	var svc service
	if err := yaml.Unmarshal(serviceYAML, &svc); err != nil {
		return fmt.Errorf("parsing service: %w", err)
	}

	if svc.Spec.Type != "NodePort" {
		return fmt.Errorf("service type is %q, expected NodePort", svc.Spec.Type)
	}

	if expectedPort < 30000 || expectedPort > 32767 {
		return fmt.Errorf("port %d outside valid NodePort range (30000-32767)", expectedPort)
	}

	for _, p := range svc.Spec.Ports {
		if p.NodePort == expectedPort {
			return nil
		}
	}

	actualPorts := make([]string, 0, len(svc.Spec.Ports))
	for _, p := range svc.Spec.Ports {
		actualPorts = append(actualPorts, fmt.Sprintf("%d", p.NodePort))
	}
	return fmt.Errorf("expected NodePort %d not found; actual ports: [%s]", expectedPort, strings.Join(actualPorts, ", "))
}
