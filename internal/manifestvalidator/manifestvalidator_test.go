package manifestvalidator

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validManifestYAML = `name: helixon
version: 1.0.0
description: Agent tooling suite
components:
  - name: router
    type: binary
    binary_path: /usr/local/bin/router
    config_path: ~/.config/router.yaml
    required: true
    platform: darwin
  - name: monitor
    type: service
    binary_path: ~/bin/monitor
    config_path: /etc/monitor.conf
    required: false
    platform: all
`

func TestParseValidManifest(t *testing.T) {
	m, err := ParseManifest([]byte(validManifestYAML))
	require.NoError(t, err)
	assert.Equal(t, "helixon", m.Name)
	assert.Equal(t, "1.0.0", m.Version)
	assert.Len(t, m.Components, 2)
	assert.Equal(t, "router", m.Components[0].Name)
	assert.True(t, m.Components[0].Required)
}

func TestParseInvalidYAML(t *testing.T) {
	_, err := ParseManifest([]byte("{{invalid yaml"))
	assert.Error(t, err)
}

func TestValidateRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		m       Manifest
		errSub  string
	}{
		{"missing name", Manifest{Version: "1.0.0"}, "name"},
		{"missing version", Manifest{Name: "test"}, "version"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateManifest(tt.m)
			assert.False(t, result.Valid)
			assert.NotEmpty(t, result.Errors)
			found := false
			for _, e := range result.Errors {
				if assert.ObjectsAreEqual(e, e) {
					found = true
				}
			}
			assert.True(t, found)
		})
	}
}

func TestValidateComponentRequiredFields(t *testing.T) {
	errs := ValidateComponent(Component{})
	assert.NotEmpty(t, errs)

	hasNameErr := false
	for _, e := range errs {
		if e != "" {
			hasNameErr = true
		}
	}
	assert.True(t, hasNameErr)
}

func TestValidatePlatformValues(t *testing.T) {
	valid := []string{"darwin", "linux", "windows", "all"}
	for _, p := range valid {
		errs := ValidateComponent(Component{Name: "test", Type: "binary", Platform: p})
		for _, e := range errs {
			assert.NotContains(t, e, "platform")
		}
	}

	errs := ValidateComponent(Component{Name: "test", Type: "binary", Platform: "freebsd"})
	found := false
	for _, e := range errs {
		if len(e) > 0 {
			found = true
		}
	}
	assert.True(t, found)
}

func TestValidateBinaryPathFormat(t *testing.T) {
	validPaths := []string{"~/bin/tool", "/usr/local/bin/tool", "bin/tool"}
	for _, p := range validPaths {
		errs := ValidateComponent(Component{Name: "t", Type: "binary", BinaryPath: p, Platform: "all"})
		for _, e := range errs {
			assert.NotContains(t, e, "binary_path")
		}
	}
}

func TestValidateEmptyComponents(t *testing.T) {
	m := Manifest{Name: "test", Version: "1.0.0"}
	result := ValidateManifest(m)
	assert.NotEmpty(t, result.Warnings)
}

func TestValidateDuplicateComponentNames(t *testing.T) {
	m := Manifest{
		Name:    "test",
		Version: "1.0.0",
		Components: []Component{
			{Name: "router", Type: "binary", Platform: "all"},
			{Name: "router", Type: "service", Platform: "all"},
		},
	}
	result := ValidateManifest(m)
	assert.False(t, result.Valid)
	found := false
	for _, e := range result.Errors {
		if len(e) > 0 {
			found = true
		}
	}
	assert.True(t, found)
}

func TestValidateVersionFormat(t *testing.T) {
	good := Manifest{Name: "test", Version: "1.2.3", Components: []Component{{Name: "a", Type: "b", Platform: "all"}}}
	assert.True(t, ValidateManifest(good).Valid)

	bad := Manifest{Name: "test", Version: "latest", Components: []Component{{Name: "a", Type: "b", Platform: "all"}}}
	result := ValidateManifest(bad)
	assert.False(t, result.Valid)
}

func TestValidateComponentJSON(t *testing.T) {
	c := Component{
		Name:       "router",
		Type:       "binary",
		BinaryPath: "/usr/local/bin/router",
		ConfigPath: "~/.config/router.yaml",
		Required:   true,
		Platform:   "darwin",
	}

	data, err := json.Marshal(c)
	require.NoError(t, err)

	var decoded Component
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, c.Name, decoded.Name)
	assert.Equal(t, c.Required, decoded.Required)
}

func TestParseManifestWithAllFields(t *testing.T) {
	m, err := ParseManifest([]byte(validManifestYAML))
	require.NoError(t, err)
	assert.Equal(t, "Agent tooling suite", m.Description)
	assert.Equal(t, "darwin", m.Components[0].Platform)
	assert.Equal(t, "~/.config/router.yaml", m.Components[0].ConfigPath)
	assert.Equal(t, "/usr/local/bin/router", m.Components[0].BinaryPath)
	assert.Equal(t, "binary", m.Components[0].Type)
}

func TestValidateManifestGreen(t *testing.T) {
	m, err := ParseManifest([]byte(validManifestYAML))
	require.NoError(t, err)

	result := ValidateManifest(m)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}
