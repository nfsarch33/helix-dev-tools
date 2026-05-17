package dockerimagevalidator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateImageReferenceAcceptsPinnedReferences(t *testing.T) {
	tests := []string{
		"ghcr.io/nfsarch33/helixon-api:1.2.3",
		"registry.example.com/team/service@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}

	for _, ref := range tests {
		t.Run(ref, func(t *testing.T) {
			result := ValidateImageReference(ref)
			assert.True(t, result.Valid)
			assert.Empty(t, result.Errors)
		})
	}
}

func TestValidateImageReferenceRejectsEmptyReference(t *testing.T) {
	result := ValidateImageReference("")

	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Errors)
}

func TestValidateImageReferenceWarnsOnLatestTag(t *testing.T) {
	result := ValidateImageReference("ghcr.io/nfsarch33/helixon-api:latest")

	assert.True(t, result.Valid)
	assert.NotEmpty(t, result.Warnings)
}

func TestValidateImageReferenceRejectsUnqualifiedName(t *testing.T) {
	result := ValidateImageReference("helixon-api")

	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Errors)
}

func TestValidateImageSetDetectsDuplicates(t *testing.T) {
	result := ValidateImageSet([]string{
		"ghcr.io/nfsarch33/helixon-api:1.2.3",
		"ghcr.io/nfsarch33/helixon-api:1.2.3",
	})

	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Errors)
}
