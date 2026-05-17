package terraformvalidator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validTerraformHCL = `terraform {
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

func TestValidateHCLSyntax_Valid(t *testing.T) {
	reqs := DefaultHCLRequirements()
	result, err := ValidateHCLSyntax(validTerraformHCL, reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "valid HCL should pass; failures: %v", result.Failures)
	assert.NotEmpty(t, result.Passed)
}

func TestValidateHCLSyntax_MissingTerraformBlock(t *testing.T) {
	hcl := `provider "google" {
  project = "helixon-platform"
  region  = "australia-southeast1"
}
`
	reqs := DefaultHCLRequirements()
	result, err := ValidateHCLSyntax(hcl, reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "missing terraform block should fail")
}

func TestValidateHCLSyntax_MissingProvider(t *testing.T) {
	hcl := `terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
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
`
	reqs := DefaultHCLRequirements()
	result, err := ValidateHCLSyntax(hcl, reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "missing google-beta provider should fail")
}

func TestValidateHCLSyntax_Empty(t *testing.T) {
	reqs := DefaultHCLRequirements()
	result, err := ValidateHCLSyntax("", reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "empty HCL should fail")
}

func TestValidateProviderConfig_Valid(t *testing.T) {
	reqs := DefaultProviderRequirements()
	result, err := ValidateProviderConfig(validTerraformHCL, reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "valid provider config should pass; failures: %v", result.Failures)
}

func TestValidateProviderConfig_NoVersionConstraint(t *testing.T) {
	hcl := `terraform {
  required_providers {
    google = {
      source = "hashicorp/google"
    }
    google-beta = {
      source = "hashicorp/google-beta"
    }
  }
}
provider "google" {
  project = "helixon-platform"
}
provider "google-beta" {
  project = "helixon-platform"
}
`
	reqs := DefaultProviderRequirements()
	result, err := ValidateProviderConfig(hcl, reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "missing version constraints should fail")
}

func TestValidateProviderConfig_NoImpersonation(t *testing.T) {
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
	reqs := DefaultProviderRequirements()
	result, err := ValidateProviderConfig(hcl, reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "missing SA impersonation should fail")
}

func TestValidateBackendConfig_Valid(t *testing.T) {
	reqs := DefaultBackendRequirements()
	result, err := ValidateBackendConfig(validTerraformHCL, reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "valid backend config should pass; failures: %v", result.Failures)
}

func TestValidateBackendConfig_WrongBucket(t *testing.T) {
	hcl := `terraform {
  backend "gcs" {
    bucket = "wrong-bucket-name"
    prefix = "terraform/state"
  }
}
`
	reqs := DefaultBackendRequirements()
	result, err := ValidateBackendConfig(hcl, reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "wrong bucket name should fail")
}

func TestValidateBackendConfig_WrongType(t *testing.T) {
	hcl := `terraform {
  backend "s3" {
    bucket = "helixon-platform-tf-state"
  }
}
`
	reqs := DefaultBackendRequirements()
	result, err := ValidateBackendConfig(hcl, reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "S3 backend instead of GCS should fail")
}

func TestValidateBackendConfig_NoBackend(t *testing.T) {
	hcl := `terraform {
  required_version = ">= 1.5.0"
}
`
	reqs := DefaultBackendRequirements()
	result, err := ValidateBackendConfig(hcl, reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "missing backend block should fail")
}
