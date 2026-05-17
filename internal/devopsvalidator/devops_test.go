package devopsvalidator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validAgentConfigYAML = `agent:
  name: devops-agent
  version: "1.0.0"
  heartbeat:
    interval: 30s
    llm_tier: local
  llm_routing:
    tiers:
      - name: local
        endpoint: http://llm-router:9787/v1
        model: qwen3.5-0.6b
        purpose: heartbeat
      - name: minimax
        endpoint: https://api.minimaxi.chat/v1
        model: MiniMax-M1
        purpose: complex-reasoning
      - name: cursor
        endpoint: https://api.cursor.com/v1
        model: claude-sonnet
        purpose: code-generation
  memory:
    provider: mem0
    endpoint: http://mem0:8080
    app_id: devops-agent
    user_id: fleet-agent
`

func TestParseAgentConfig(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg, err := ParseAgentConfig([]byte(validAgentConfigYAML))
		require.NoError(t, err)
		assert.Equal(t, "devops-agent", cfg.Agent.Name)
		assert.Equal(t, "1.0.0", cfg.Agent.Version)
		assert.Len(t, cfg.Agent.LLMRouting.Tiers, 3)
	})

	t.Run("empty input", func(t *testing.T) {
		_, err := ParseAgentConfig([]byte(""))
		assert.Error(t, err)
	})

	t.Run("invalid yaml", func(t *testing.T) {
		_, err := ParseAgentConfig([]byte("{{invalid"))
		assert.Error(t, err)
	})
}

func TestValidateLLMRouting(t *testing.T) {
	t.Run("valid three-tier routing", func(t *testing.T) {
		cfg, err := ParseAgentConfig([]byte(validAgentConfigYAML))
		require.NoError(t, err)
		err = ValidateLLMRouting(cfg)
		assert.NoError(t, err)
	})

	t.Run("missing local tier", func(t *testing.T) {
		yaml := `agent:
  name: devops-agent
  llm_routing:
    tiers:
      - name: minimax
        endpoint: https://api.minimaxi.chat/v1
        model: MiniMax-M1
        purpose: complex-reasoning
`
		cfg, err := ParseAgentConfig([]byte(yaml))
		require.NoError(t, err)
		err = ValidateLLMRouting(cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "local")
	})

	t.Run("tier missing endpoint", func(t *testing.T) {
		yaml := `agent:
  name: devops-agent
  llm_routing:
    tiers:
      - name: local
        model: qwen3.5-0.6b
        purpose: heartbeat
`
		cfg, err := ParseAgentConfig([]byte(yaml))
		require.NoError(t, err)
		err = ValidateLLMRouting(cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "endpoint")
	})

	t.Run("no tiers defined", func(t *testing.T) {
		yaml := `agent:
  name: devops-agent
  llm_routing:
    tiers: []
`
		cfg, err := ParseAgentConfig([]byte(yaml))
		require.NoError(t, err)
		err = ValidateLLMRouting(cfg)
		assert.Error(t, err)
	})
}

func TestValidateMemoryConfig(t *testing.T) {
	t.Run("valid Mem0 config", func(t *testing.T) {
		cfg, err := ParseAgentConfig([]byte(validAgentConfigYAML))
		require.NoError(t, err)
		err = ValidateMemoryConfig(cfg)
		assert.NoError(t, err)
	})

	t.Run("missing endpoint", func(t *testing.T) {
		yaml := `agent:
  name: devops-agent
  memory:
    provider: mem0
    app_id: devops-agent
`
		cfg, err := ParseAgentConfig([]byte(yaml))
		require.NoError(t, err)
		err = ValidateMemoryConfig(cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "endpoint")
	})

	t.Run("missing app_id", func(t *testing.T) {
		yaml := `agent:
  name: devops-agent
  memory:
    provider: mem0
    endpoint: http://mem0:8080
`
		cfg, err := ParseAgentConfig([]byte(yaml))
		require.NoError(t, err)
		err = ValidateMemoryConfig(cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "app_id")
	})
}

func TestValidateHeartbeat(t *testing.T) {
	t.Run("valid heartbeat", func(t *testing.T) {
		cfg, err := ParseAgentConfig([]byte(validAgentConfigYAML))
		require.NoError(t, err)
		err = ValidateHeartbeat(cfg)
		assert.NoError(t, err)
	})

	t.Run("missing interval", func(t *testing.T) {
		yaml := `agent:
  name: devops-agent
  heartbeat:
    llm_tier: local
`
		cfg, err := ParseAgentConfig([]byte(yaml))
		require.NoError(t, err)
		err = ValidateHeartbeat(cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "interval")
	})

	t.Run("missing llm_tier", func(t *testing.T) {
		yaml := `agent:
  name: devops-agent
  heartbeat:
    interval: 30s
`
		cfg, err := ParseAgentConfig([]byte(yaml))
		require.NoError(t, err)
		err = ValidateHeartbeat(cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "llm_tier")
	})
}

func TestValidateResourceLimits(t *testing.T) {
	t.Run("within limits", func(t *testing.T) {
		yaml := `apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: devops-agent
spec:
  template:
    spec:
      containers:
        - name: agent
          image: helixon/devops-agent:1.0.0
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 500m
              memory: 512Mi
`
		err := ValidateResourceLimits([]byte(yaml))
		assert.NoError(t, err)
	})

	t.Run("exceeds memory limit", func(t *testing.T) {
		yaml := `apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: devops-agent
spec:
  template:
    spec:
      containers:
        - name: agent
          image: helixon/devops-agent:1.0.0
          resources:
            limits:
              cpu: 500m
              memory: 1Gi
`
		err := ValidateResourceLimits([]byte(yaml))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "memory")
	})

	t.Run("exceeds cpu limit", func(t *testing.T) {
		yaml := `apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: devops-agent
spec:
  template:
    spec:
      containers:
        - name: agent
          image: helixon/devops-agent:1.0.0
          resources:
            limits:
              cpu: "1"
              memory: 256Mi
`
		err := ValidateResourceLimits([]byte(yaml))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cpu")
	})

	t.Run("no resource limits", func(t *testing.T) {
		yaml := `apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: devops-agent
spec:
  template:
    spec:
      containers:
        - name: agent
          image: helixon/devops-agent:1.0.0
`
		err := ValidateResourceLimits([]byte(yaml))
		assert.Error(t, err)
	})
}
