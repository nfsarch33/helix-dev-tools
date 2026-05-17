package terraformvalidator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- QA: HCL validation accuracy ---

func TestQA_HCLSyntax_WithComments(t *testing.T) {
	hcl := `# Main infrastructure config
terraform {
  required_version = ">= 1.5.0"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.0"
    }
    google-beta = {
      source  = "hashicorp/google-beta"
      version = "~> 5.0"
    }
  }

  backend "gcs" {
    bucket = "helixon-platform-tf-state"
    prefix = "terraform/state"
  }
}

provider "google" {
  project = "helixon-platform"
  region  = "australia-southeast1"
}

provider "google-beta" {
  project = "helixon-platform"
  region  = "australia-southeast1"
}
`
	reqs := DefaultHCLRequirements()
	result, err := ValidateHCLSyntax(hcl, reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "HCL with comments should still pass; failures: %v", result.Failures)
}

func TestQA_HCLSyntax_OnlyLocalBackend(t *testing.T) {
	hcl := `terraform {
  backend "local" {
    path = "terraform.tfstate"
  }
}
provider "google" {
  project = "helixon-platform"
}
provider "google-beta" {
  project = "helixon-platform"
}
`
	reqs := DefaultHCLRequirements()
	result, err := ValidateHCLSyntax(hcl, reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "local backend should fail GCS requirement")
}

func TestQA_HCLSyntax_MultipleModules(t *testing.T) {
	hcl := `terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.0"
    }
    google-beta = {
      source  = "hashicorp/google-beta"
      version = "~> 5.0"
    }
  }
  backend "gcs" {
    bucket = "helixon-platform-tf-state"
  }
}

provider "google" {
  project = "helixon-platform"
}

provider "google-beta" {
  project = "helixon-platform"
}

module "networking" {
  source = "./modules/networking"
}
`
	reqs := DefaultHCLRequirements()
	result, err := ValidateHCLSyntax(hcl, reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "HCL with modules should still pass structural checks")
}

// --- QA: Provider version constraints ---

func TestQA_ProviderConfig_PinnedVersion(t *testing.T) {
	hcl := `terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "= 5.30.0"
    }
    google-beta = {
      source  = "hashicorp/google-beta"
      version = "= 5.30.0"
    }
  }
}
provider "google" {
  project                     = "helixon-platform"
  impersonate_service_account = "tf-bootstrap@helixon-platform.iam.gserviceaccount.com"
}
provider "google-beta" {
  project                     = "helixon-platform"
  impersonate_service_account = "tf-bootstrap@helixon-platform.iam.gserviceaccount.com"
}
`
	reqs := DefaultProviderRequirements()
	result, err := ValidateProviderConfig(hcl, reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "pinned version should be accepted; failures: %v", result.Failures)
}

func TestQA_ProviderConfig_MinVersionConstraint(t *testing.T) {
	hcl := `terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = ">= 5.0, < 6.0"
    }
    google-beta = {
      source  = "hashicorp/google-beta"
      version = ">= 5.0, < 6.0"
    }
  }
}
provider "google" {
  project                     = "helixon-platform"
  impersonate_service_account = "tf-bootstrap@helixon-platform.iam.gserviceaccount.com"
}
provider "google-beta" {
  project                     = "helixon-platform"
  impersonate_service_account = "tf-bootstrap@helixon-platform.iam.gserviceaccount.com"
}
`
	reqs := DefaultProviderRequirements()
	result, err := ValidateProviderConfig(hcl, reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "range version constraint should pass; failures: %v", result.Failures)
}

// --- QA: State backend config ---

func TestQA_BackendConfig_WithEncryption(t *testing.T) {
	hcl := `terraform {
  backend "gcs" {
    bucket = "helixon-platform-tf-state"
    prefix = "terraform/state"
  }
}
`
	reqs := DefaultBackendRequirements()
	result, err := ValidateBackendConfig(hcl, reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "backend with prefix should pass; failures: %v", result.Failures)
}

func TestQA_BackendConfig_S3InsteadOfGCS(t *testing.T) {
	hcl := `terraform {
  backend "s3" {
    bucket = "helixon-platform-tf-state"
    key    = "terraform.tfstate"
    region = "ap-southeast-2"
  }
}
`
	reqs := DefaultBackendRequirements()
	result, err := ValidateBackendConfig(hcl, reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "S3 backend should fail GCS requirement")
}

func TestQA_BackendConfig_ConsulBackend(t *testing.T) {
	hcl := `terraform {
  backend "consul" {
    address = "demo.consul.io"
    path    = "full/path"
  }
}
`
	reqs := DefaultBackendRequirements()
	result, err := ValidateBackendConfig(hcl, reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "Consul backend should fail GCS requirement")
}

// --- QA: No-JSON-key policy enforcement ---

func TestQA_NoJSONKeys_WithGOOGLE_APPLICATION_CREDENTIALS(t *testing.T) {
	hcl := `# Export GOOGLE_APPLICATION_CREDENTIALS=/path/to/key.json before running
provider "google" {
  project = "helixon-platform"
}
`
	result, err := ValidateNoJSONKeys(hcl)
	require.NoError(t, err)
	assert.False(t, result.OK(), "GOOGLE_APPLICATION_CREDENTIALS comment reference should fail")
}

func TestQA_NoJSONKeys_CleanImpersonation(t *testing.T) {
	hcl := `provider "google" {
  project                     = "helixon-platform"
  region                      = "australia-southeast1"
  impersonate_service_account = "tf-bootstrap@helixon-platform.iam.gserviceaccount.com"
}

provider "google-beta" {
  project                     = "helixon-platform"
  region                      = "australia-southeast1"
  impersonate_service_account = "tf-bootstrap@helixon-platform.iam.gserviceaccount.com"
}
`
	result, err := ValidateNoJSONKeys(hcl)
	require.NoError(t, err)
	assert.True(t, result.OK(), "pure impersonation config should pass; failures: %v", result.Failures)
}

func TestQA_NoJSONKeys_CredFileInVariable(t *testing.T) {
	hcl := `variable "google_credentials" {
  type    = string
  default = "credentials.json"
}

provider "google" {
  credentials = var.google_credentials
  project     = "helixon-platform"
}
`
	result, err := ValidateNoJSONKeys(hcl)
	require.NoError(t, err)
	assert.False(t, result.OK(), "credentials field even via variable should fail")
}

func TestQA_GCPProject_FullValidConfig(t *testing.T) {
	reqs := DefaultGCPProjectRequirements()
	result, err := ValidateGCPProject(validTerraformHCL, reqs)
	require.NoError(t, err)
	assert.True(t, result.OK())
	assert.GreaterOrEqual(t, len(result.Passed), 4, "should have at least 4 passed checks")
}

func TestQA_GCPProject_MissingProject(t *testing.T) {
	hcl := `provider "google" {
  region = "australia-southeast1"
}
terraform {
  backend "gcs" {
    bucket = "helixon-platform-tf-state"
  }
}
`
	reqs := DefaultGCPProjectRequirements()
	result, err := ValidateGCPProject(hcl, reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "missing project field should fail")
}

func TestQA_GCPProject_MissingRegion(t *testing.T) {
	hcl := `provider "google" {
  project = "helixon-platform"
}
terraform {
  backend "gcs" {
    bucket = "helixon-platform-tf-state"
  }
}
`
	reqs := DefaultGCPProjectRequirements()
	result, err := ValidateGCPProject(hcl, reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "missing region field should fail")
}
