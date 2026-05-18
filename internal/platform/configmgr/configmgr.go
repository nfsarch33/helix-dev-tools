package configmgr

import (
	"fmt"
	"sort"
)

// Config holds key-value configuration entries
type Config struct {
	values map[string]string
}

// New creates an empty Config
func New() *Config {
	return &Config{values: map[string]string{}}
}

// Set stores a configuration value
func (c *Config) Set(key, value string) {
	c.values[key] = value
}

// Get returns a configuration value and whether it exists
func (c *Config) Get(key string) (string, bool) {
	v, ok := c.values[key]
	return v, ok
}

// All returns all key-value pairs sorted by key
func (c *Config) All() map[string]string {
	result := make(map[string]string, len(c.values))
	for k, v := range c.values {
		result[k] = v
	}
	return result
}

// Diff returns keys that differ between c and other
func (c *Config) Diff(other *Config) []string {
	seen := map[string]struct{}{}
	var diffs []string

	for k, v := range c.values {
		seen[k] = struct{}{}
		ov, ok := other.values[k]
		if !ok || ov != v {
			diffs = append(diffs, k)
		}
	}
	for k := range other.values {
		if _, ok := seen[k]; !ok {
			diffs = append(diffs, k)
		}
	}
	sort.Strings(diffs)
	return diffs
}

// Validate checks that all required keys are present and non-empty
func (c *Config) Validate(required []string) []error {
	var errs []error
	for _, k := range required {
		v, ok := c.values[k]
		if !ok || v == "" {
			errs = append(errs, fmt.Errorf("required config key %q is missing or empty", k))
		}
	}
	return errs
}

// Merge copies all values from other into c, overwriting on conflict
func (c *Config) Merge(other *Config) {
	for k, v := range other.values {
		c.values[k] = v
	}
}
