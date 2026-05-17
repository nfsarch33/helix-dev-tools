package devopsvalidator

// AgentConfig represents the top-level devops agent configuration.
type AgentConfig struct {
	Agent AgentSpec `yaml:"agent"`
}

// AgentSpec defines the agent's operational parameters.
type AgentSpec struct {
	Name       string          `yaml:"name"`
	Version    string          `yaml:"version"`
	Heartbeat  HeartbeatConfig `yaml:"heartbeat"`
	LLMRouting LLMRouting      `yaml:"llm_routing"`
	Memory     MemoryConfig    `yaml:"memory"`
}

// HeartbeatConfig defines the heartbeat check interval and LLM tier.
type HeartbeatConfig struct {
	Interval string `yaml:"interval"`
	LLMTier  string `yaml:"llm_tier"`
}

// LLMRouting holds the tiered LLM routing configuration.
type LLMRouting struct {
	Tiers []LLMTier `yaml:"tiers"`
}

// LLMTier represents a single LLM routing tier (local, minimax, cursor).
type LLMTier struct {
	Name     string `yaml:"name"`
	Endpoint string `yaml:"endpoint"`
	Model    string `yaml:"model"`
	Purpose  string `yaml:"purpose"`
}

// MemoryConfig holds the agent's memory backend configuration.
type MemoryConfig struct {
	Provider string `yaml:"provider"`
	Endpoint string `yaml:"endpoint"`
	AppID    string `yaml:"app_id"`
	UserID   string `yaml:"user_id"`
}
