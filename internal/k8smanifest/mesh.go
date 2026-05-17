package k8smanifest

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// ValidateServiceMesh checks Ingress manifests against service mesh requirements.
func ValidateServiceMesh(data []byte, reqs ServiceMeshRequirements) (*ValidationResult, error) {
	docs, err := splitYAMLDocs(data)
	if err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}

	var ingress *ManifestDoc
	for i := range docs {
		if docs[i].Kind == "Ingress" {
			ingress = &docs[i]
			break
		}
	}
	if ingress == nil {
		return nil, fmt.Errorf("no Ingress found in manifest")
	}

	result := &ValidationResult{Name: "ServiceMesh-Ingress"}

	if reqs.RequireTLS {
		tlsOK := checkTLSBlock(ingress)
		if tlsOK {
			result.Passed = append(result.Passed, "TLS termination configured")
		} else {
			result.Failures = append(result.Failures, "TLS termination not configured (required)")
		}
	} else {
		result.Passed = append(result.Passed, "TLS not required, skipped")
	}

	hostOK := checkIngressHost(ingress)
	if hostOK {
		result.Passed = append(result.Passed, "ingress host specified")
	} else {
		result.Failures = append(result.Failures, "ingress host not specified")
	}

	pathOK := checkIngressPath(ingress)
	if pathOK {
		result.Passed = append(result.Passed, "ingress path configured")
	} else {
		result.Failures = append(result.Failures, "ingress path not configured")
	}

	return result, nil
}

// ValidateNetworkPolicy checks NetworkPolicy manifests.
func ValidateNetworkPolicy(data []byte, reqs ServiceMeshRequirements) (*ValidationResult, error) {
	var doc struct {
		APIVersion string `yaml:"apiVersion"`
		Kind       string `yaml:"kind"`
		Metadata   struct {
			Name      string `yaml:"name"`
			Namespace string `yaml:"namespace"`
		} `yaml:"metadata"`
		Spec struct {
			PodSelector any      `yaml:"podSelector"`
			PolicyTypes []string `yaml:"policyTypes"`
			Ingress     []any    `yaml:"ingress"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing NetworkPolicy YAML: %w", err)
	}

	result := &ValidationResult{Name: "ServiceMesh-NetworkPolicy"}

	if doc.Kind != "NetworkPolicy" {
		result.Failures = append(result.Failures, fmt.Sprintf("expected NetworkPolicy, got %s", doc.Kind))
		return result, nil
	}
	result.Passed = append(result.Passed, "kind is NetworkPolicy")

	if len(doc.Spec.Ingress) == 0 {
		result.Failures = append(result.Failures, "no ingress rules defined")
	} else {
		result.Passed = append(result.Passed, fmt.Sprintf("%d ingress rule(s) defined", len(doc.Spec.Ingress)))
	}

	hasIngressType := false
	for _, pt := range doc.Spec.PolicyTypes {
		if pt == "Ingress" {
			hasIngressType = true
		}
	}
	if hasIngressType {
		result.Passed = append(result.Passed, "policyTypes includes Ingress")
	} else {
		result.Failures = append(result.Failures, "policyTypes missing Ingress")
	}

	return result, nil
}

func checkTLSBlock(doc *ManifestDoc) bool {
	spec := doc.Spec
	tls, ok := spec["tls"]
	if !ok {
		return false
	}
	tlsList, ok := tls.([]any)
	if !ok {
		return false
	}
	return len(tlsList) > 0
}

func checkIngressHost(doc *ManifestDoc) bool {
	rules, ok := doc.Spec["rules"]
	if !ok {
		return false
	}
	ruleList, ok := rules.([]any)
	if !ok || len(ruleList) == 0 {
		return false
	}
	for _, r := range ruleList {
		rm, ok := r.(map[string]any)
		if !ok {
			continue
		}
		if host, ok := rm["host"]; ok {
			hostStr := fmt.Sprintf("%v", host)
			if hostStr != "" && hostStr != "<nil>" {
				return true
			}
		}
	}
	return false
}

func checkIngressPath(doc *ManifestDoc) bool {
	rules, ok := doc.Spec["rules"]
	if !ok {
		return false
	}
	ruleList, ok := rules.([]any)
	if !ok || len(ruleList) == 0 {
		return false
	}
	for _, r := range ruleList {
		rm, ok := r.(map[string]any)
		if !ok {
			continue
		}
		http, ok := rm["http"]
		if !ok {
			continue
		}
		hm, ok := http.(map[string]any)
		if !ok {
			continue
		}
		paths, ok := hm["paths"]
		if !ok {
			continue
		}
		pathList, ok := paths.([]any)
		if !ok {
			continue
		}
		if len(pathList) > 0 {
			return true
		}
	}
	return false
}
