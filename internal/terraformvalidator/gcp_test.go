package terraformvalidator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateGCPProject_Valid(t *testing.T) {
	reqs := DefaultGCPProjectRequirements()
	result, err := ValidateGCPProject(validTerraformHCL, reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "valid GCP project config should pass; failures: %v", result.Failures)
}

func TestValidateGCPProject_WrongProjectID(t *testing.T) {
	hcl := `provider "google" {
  project                     = "wrong-project"
  region                      = "australia-southeast1"
  impersonate_service_account = "tf-bootstrap@helixon-platform.iam.gserviceaccount.com"
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
	assert.False(t, result.OK(), "wrong project ID should fail")
}

func TestValidateGCPProject_WrongRegion(t *testing.T) {
	hcl := `provider "google" {
  project                     = "helixon-platform"
  region                      = "us-central1"
  impersonate_service_account = "tf-bootstrap@helixon-platform.iam.gserviceaccount.com"
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
	assert.False(t, result.OK(), "wrong region should fail")
}

func TestValidateGCPProject_WrongStateBucket(t *testing.T) {
	hcl := `provider "google" {
  project = "helixon-platform"
  region  = "australia-southeast1"
}
terraform {
  backend "gcs" {
    bucket = "wrong-bucket"
  }
}
`
	reqs := DefaultGCPProjectRequirements()
	result, err := ValidateGCPProject(hcl, reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "wrong state bucket should fail")
}

func TestValidateGCPProject_ContainsJSONKeyRef(t *testing.T) {
	hcl := `provider "google" {
  project     = "helixon-platform"
  region      = "australia-southeast1"
  credentials = file("service-account-key.json")
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
	assert.False(t, result.OK(), "JSON key file reference should fail")
}

func TestValidateGCPProject_ContainsCredentialsField(t *testing.T) {
	hcl := `provider "google" {
  project     = "helixon-platform"
  region      = "australia-southeast1"
  credentials = "/path/to/creds.json"
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
	assert.False(t, result.OK(), "credentials field reference should fail")
}

func TestValidateGCPProject_GOOGLE_CREDENTIALS_EnvRef(t *testing.T) {
	hcl := `provider "google" {
  project = "helixon-platform"
  region  = "australia-southeast1"
}
# GOOGLE_CREDENTIALS = "/path/to/key.json"
terraform {
  backend "gcs" {
    bucket = "helixon-platform-tf-state"
  }
}
`
	reqs := DefaultGCPProjectRequirements()
	result, err := ValidateGCPProject(hcl, reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "GOOGLE_CREDENTIALS env reference should fail")
}

func TestValidateNoJSONKeys_Clean(t *testing.T) {
	hcl := `provider "google" {
  project                     = "helixon-platform"
  region                      = "australia-southeast1"
  impersonate_service_account = "tf-bootstrap@helixon-platform.iam.gserviceaccount.com"
}
`
	result, err := ValidateNoJSONKeys(hcl)
	require.NoError(t, err)
	assert.True(t, result.OK(), "clean config should pass; failures: %v", result.Failures)
}

func TestValidateNoJSONKeys_WithKeyFile(t *testing.T) {
	hcl := `provider "google" {
  credentials = file("sa-key.json")
}
`
	result, err := ValidateNoJSONKeys(hcl)
	require.NoError(t, err)
	assert.False(t, result.OK(), "JSON key file reference should fail")
}

func TestValidateNoJSONKeys_WithAccessToken(t *testing.T) {
	hcl := `provider "google" {
  access_token = "ya29.hardcoded-token-value"
}
`
	result, err := ValidateNoJSONKeys(hcl)
	require.NoError(t, err)
	assert.False(t, result.OK(), "hardcoded access_token should fail")
}
