package k8smanifest

// ManifestDoc represents a single YAML document parsed from a multi-doc manifest.
type ManifestDoc struct {
	APIVersion string            `yaml:"apiVersion"`
	Kind       string            `yaml:"kind"`
	Metadata   ManifestMeta      `yaml:"metadata"`
	Spec       map[string]any    `yaml:"spec"`
	Data       map[string]string `yaml:"data,omitempty"`
	Raw        []byte            `yaml:"-"`
}

// ManifestMeta is the metadata block shared by all K8s resources.
type ManifestMeta struct {
	Name        string            `yaml:"name"`
	Namespace   string            `yaml:"namespace"`
	Labels      map[string]string `yaml:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty"`
}

// ValidationResult collects pass/fail findings from a manifest check.
type ValidationResult struct {
	Name     string
	Passed   []string
	Failures []string
}

func (v *ValidationResult) OK() bool { return len(v.Failures) == 0 }

// OmniParserRequirements specifies expected values for OmniParser deployment validation.
type OmniParserRequirements struct {
	MinGPU           int
	RequiredProbes   []string
	ModelVolumePath  string
	MinMemoryLimitMi int
	Namespace        string
}

// DefaultOmniParserRequirements returns sensible defaults for OmniParser 2 deployments.
func DefaultOmniParserRequirements() OmniParserRequirements {
	return OmniParserRequirements{
		MinGPU:           1,
		RequiredProbes:   []string{"livenessProbe", "readinessProbe"},
		ModelVolumePath:  "/models",
		MinMemoryLimitMi: 4096,
		Namespace:        "uiauto",
	}
}

// ServiceMeshRequirements specifies expected service mesh / ingress properties.
type ServiceMeshRequirements struct {
	RequireTLS            bool
	AllowedNamespaces     []string
	RequiredIngressFields []string
}

// DefaultServiceMeshRequirements returns sensible defaults.
func DefaultServiceMeshRequirements() ServiceMeshRequirements {
	return ServiceMeshRequirements{
		RequireTLS:            true,
		AllowedNamespaces:     []string{"uiauto", "monitoring", "llm-cluster"},
		RequiredIngressFields: []string{"host", "path"},
	}
}
