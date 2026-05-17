package loggingvalidator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateLogQLSyntax_ValidStream(t *testing.T) {
	query := `{namespace="logging", pod=~"loki.*"}`
	result, err := ValidateLogQLSyntax(query)
	require.NoError(t, err)
	assert.True(t, result.OK(), "valid LogQL stream selector should pass; failures: %v", result.Failures)
}

func TestValidateLogQLSyntax_ValidWithFilter(t *testing.T) {
	query := `{namespace="logging"} |= "error" | json | level="error"`
	result, err := ValidateLogQLSyntax(query)
	require.NoError(t, err)
	assert.True(t, result.OK(), "valid LogQL with filter should pass; failures: %v", result.Failures)
}

func TestValidateLogQLSyntax_ValidRate(t *testing.T) {
	query := `rate({namespace="logging"}[5m])`
	result, err := ValidateLogQLSyntax(query)
	require.NoError(t, err)
	assert.True(t, result.OK(), "valid rate query should pass; failures: %v", result.Failures)
}

func TestValidateLogQLSyntax_EmptyQuery(t *testing.T) {
	result, err := ValidateLogQLSyntax("")
	require.NoError(t, err)
	assert.False(t, result.OK(), "empty query should fail")
}

func TestValidateLogQLSyntax_NoStreamSelector(t *testing.T) {
	query := `|= "error"`
	result, err := ValidateLogQLSyntax(query)
	require.NoError(t, err)
	assert.False(t, result.OK(), "query without stream selector should fail")
}

func TestValidateLogQLSyntax_UnbalancedBraces(t *testing.T) {
	query := `{namespace="logging"`
	result, err := ValidateLogQLSyntax(query)
	require.NoError(t, err)
	assert.False(t, result.OK(), "unbalanced braces should fail")
}

func TestValidateLogQLSyntax_UnbalancedParens(t *testing.T) {
	query := `rate({namespace="logging"}[5m]`
	result, err := ValidateLogQLSyntax(query)
	require.NoError(t, err)
	assert.False(t, result.OK(), "unbalanced parentheses should fail")
}

func TestValidateLabelMatching_AllPresent(t *testing.T) {
	query := `{namespace="logging", pod=~"loki.*"}`
	reqs := DefaultLogQueryRequirements()
	result, err := ValidateLabelMatching(query, reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "query with required labels should pass; failures: %v", result.Failures)
}

func TestValidateLabelMatching_MissingNamespace(t *testing.T) {
	query := `{pod=~"loki.*"}`
	reqs := DefaultLogQueryRequirements()
	result, err := ValidateLabelMatching(query, reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "query missing namespace label should fail")
}

func TestValidateLabelMatching_MissingPod(t *testing.T) {
	query := `{namespace="logging"}`
	reqs := DefaultLogQueryRequirements()
	result, err := ValidateLabelMatching(query, reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "query missing pod label should fail")
}

func TestValidateRetentionCompliance_WithinRange(t *testing.T) {
	cfg := `limits_config:
  retention_period: 168h
compactor:
  retention_enabled: true
`
	reqs := DefaultLogRetentionRequirements()
	result, err := ValidateRetentionCompliance([]byte(cfg), reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "168h within 7-30 day range should pass; failures: %v", result.Failures)
}

func TestValidateRetentionCompliance_BelowMin(t *testing.T) {
	cfg := `limits_config:
  retention_period: 24h
compactor:
  retention_enabled: true
`
	reqs := DefaultLogRetentionRequirements()
	result, err := ValidateRetentionCompliance([]byte(cfg), reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "24h below 7 day minimum should fail")
}

func TestValidateRetentionCompliance_AboveMax(t *testing.T) {
	cfg := `limits_config:
  retention_period: 1440h
compactor:
  retention_enabled: true
`
	reqs := DefaultLogRetentionRequirements()
	result, err := ValidateRetentionCompliance([]byte(cfg), reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "60 days above 30 day max should fail")
}

func TestValidateRetentionCompliance_RetentionDisabled(t *testing.T) {
	cfg := `limits_config:
  retention_period: 168h
compactor:
  retention_enabled: false
`
	reqs := DefaultLogRetentionRequirements()
	result, err := ValidateRetentionCompliance([]byte(cfg), reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "disabled retention should fail compliance")
}
