package mcpfilter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type MCPServer struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Enabled bool              `json:"enabled,omitempty"`
}

type MCPConfig struct {
	MCPServers map[string]MCPServer `json:"mcpServers"`
}

type ProfileDef struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Include     []string `json:"include"`
	Exclude     []string `json:"exclude,omitempty"`
}

type FilterResult struct {
	Profile     string   `json:"profile"`
	TotalIn     int      `json:"total_in"`
	TotalOut    int      `json:"total_out"`
	Removed     []string `json:"removed"`
	Retained    []string `json:"retained"`
	ReductionPc float64  `json:"reduction_percent"`
}

func LoadMCPConfig(path string) (*MCPConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read mcp config: %w", err)
	}
	var cfg MCPConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse mcp config: %w", err)
	}
	return &cfg, nil
}

func ApplyProfile(cfg *MCPConfig, profile ProfileDef) (*MCPConfig, FilterResult) {
	result := FilterResult{
		Profile: profile.Name,
		TotalIn: len(cfg.MCPServers),
	}

	filtered := &MCPConfig{
		MCPServers: make(map[string]MCPServer),
	}

	excludeSet := make(map[string]bool)
	for _, e := range profile.Exclude {
		excludeSet[e] = true
	}

	for name, srv := range cfg.MCPServers {
		if excludeSet[name] {
			result.Removed = append(result.Removed, name)
			continue
		}

		if len(profile.Include) > 0 {
			matched := false
			for _, pattern := range profile.Include {
				if matchPattern(name, pattern) {
					matched = true
					break
				}
			}
			if !matched {
				result.Removed = append(result.Removed, name)
				continue
			}
		}

		filtered.MCPServers[name] = srv
		result.Retained = append(result.Retained, name)
	}

	sort.Strings(result.Removed)
	sort.Strings(result.Retained)
	result.TotalOut = len(filtered.MCPServers)
	if result.TotalIn > 0 {
		result.ReductionPc = float64(result.TotalIn-result.TotalOut) / float64(result.TotalIn) * 100
	}
	return filtered, result
}

func WriteMCPConfig(cfg *MCPConfig, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func matchPattern(name, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(name, strings.TrimSuffix(pattern, "*"))
	}
	return name == pattern
}
