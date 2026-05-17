package terraformvalidator

// ValidationResult collects pass/fail findings from a Terraform config check.
type ValidationResult struct {
	Name     string
	Passed   []string
	Failures []string
}

func (v *ValidationResult) OK() bool { return len(v.Failures) == 0 }

// HCLRequirements specifies expected HCL syntax properties.
type HCLRequirements struct {
	RequireProviders     []string
	RequireBackendType   string
	RequireTerraformBlock bool
}

// DefaultHCLRequirements returns defaults for the Helixon platform.
func DefaultHCLRequirements() HCLRequirements {
	return HCLRequirements{
		RequireProviders:     []string{"google", "google-beta"},
		RequireBackendType:   "gcs",
		RequireTerraformBlock: true,
	}
}

// ProviderRequirements specifies expected provider configuration.
type ProviderRequirements struct {
	RequiredProviders       []string
	RequireVersionConstraint bool
	RequireImpersonation     bool
	ImpersonationSA          string
}

// DefaultProviderRequirements returns defaults for Helixon.
func DefaultProviderRequirements() ProviderRequirements {
	return ProviderRequirements{
		RequiredProviders:       []string{"google", "google-beta"},
		RequireVersionConstraint: true,
		RequireImpersonation:     true,
		ImpersonationSA:          "tf-bootstrap@helixon-platform.iam.gserviceaccount.com",
	}
}

// BackendRequirements specifies expected backend configuration.
type BackendRequirements struct {
	Type       string
	BucketName string
	Prefix     string
}

// DefaultBackendRequirements returns defaults for Helixon.
func DefaultBackendRequirements() BackendRequirements {
	return BackendRequirements{
		Type:       "gcs",
		BucketName: "helixon-platform-tf-state",
		Prefix:     "terraform/state",
	}
}

// GCPProjectRequirements specifies expected GCP project configuration.
type GCPProjectRequirements struct {
	ProjectID         string
	Region            string
	StateBucket       string
	ForbidJSONKeys    bool
}

// DefaultGCPProjectRequirements returns defaults for Helixon.
func DefaultGCPProjectRequirements() GCPProjectRequirements {
	return GCPProjectRequirements{
		ProjectID:      "helixon-platform",
		Region:         "australia-southeast1",
		StateBucket:    "helixon-platform-tf-state",
		ForbidJSONKeys: true,
	}
}
