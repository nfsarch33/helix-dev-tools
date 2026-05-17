package helmvalidator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validChart = `apiVersion: v2
name: helixon-api
description: Helixon API chart
type: application
version: 1.2.3
appVersion: "1.2.3"
`

func TestParseChartReadsMetadata(t *testing.T) {
	chart, err := ParseChart([]byte(validChart))
	require.NoError(t, err)

	assert.Equal(t, "helixon-api", chart.Name)
	assert.Equal(t, "1.2.3", chart.Version)
}

func TestValidateChartAcceptsValidChart(t *testing.T) {
	chart, err := ParseChart([]byte(validChart))
	require.NoError(t, err)

	result := ValidateChart(chart)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}

func TestValidateChartRejectsMissingRequiredFields(t *testing.T) {
	result := ValidateChart(Chart{})

	assert.False(t, result.Valid)
	assert.GreaterOrEqual(t, len(result.Errors), 3)
}

func TestValidateChartRejectsInvalidSemver(t *testing.T) {
	result := ValidateChart(Chart{
		APIVersion: "v2",
		Name:       "helixon-api",
		Version:    "latest",
	})

	assert.False(t, result.Valid)
}

func TestParseChartRejectsInvalidYAML(t *testing.T) {
	_, err := ParseChart([]byte("{{not-yaml"))
	assert.Error(t, err)
}
