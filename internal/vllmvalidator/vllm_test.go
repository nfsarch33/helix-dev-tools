package vllmvalidator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validVLLMDeploymentYAML = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: vllm-3090
  namespace: llm
spec:
  replicas: 1
  selector:
    matchLabels:
      app: vllm-3090
  template:
    metadata:
      labels:
        app: vllm-3090
    spec:
      containers:
        - name: vllm
          image: vllm/vllm-openai:latest
          args:
            - --model=Qwen/Qwen3-8B
            - --max-model-len=8192
            - --quantization=awq
            - --gpu-memory-utilization=0.9
          ports:
            - containerPort: 8000
          resources:
            limits:
              nvidia.com/gpu: "1"
          readinessProbe:
            httpGet:
              path: /health
              port: 8000
            initialDelaySeconds: 60
            periodSeconds: 10
          volumeMounts:
            - name: model-cache
              mountPath: /root/.cache/huggingface
      volumes:
        - name: model-cache
          hostPath:
            path: /mnt/models
`

func TestValidateVLLMDeployment(t *testing.T) {
	t.Run("valid deployment", func(t *testing.T) {
		err := ValidateVLLMDeployment([]byte(validVLLMDeploymentYAML))
		assert.NoError(t, err)
	})

	t.Run("missing GPU request", func(t *testing.T) {
		yaml := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: vllm-cpu
spec:
  template:
    spec:
      containers:
        - name: vllm
          image: vllm/vllm-openai:latest
          readinessProbe:
            httpGet:
              path: /health
              port: 8000
`
		err := ValidateVLLMDeployment([]byte(yaml))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "GPU")
	})

	t.Run("missing health probe", func(t *testing.T) {
		yaml := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: vllm-noprobe
spec:
  template:
    spec:
      containers:
        - name: vllm
          image: vllm/vllm-openai:latest
          resources:
            limits:
              nvidia.com/gpu: "1"
`
		err := ValidateVLLMDeployment([]byte(yaml))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "health probe")
	})
}

func TestValidateModelConfig(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg := &VLLMConfig{
			Model:        "Qwen/Qwen3-8B",
			Quantization: "awq",
			MaxModelLen:  8192,
		}
		err := ValidateModelConfig(cfg)
		assert.NoError(t, err)
	})

	t.Run("missing model name", func(t *testing.T) {
		cfg := &VLLMConfig{
			Quantization: "awq",
			MaxModelLen:  8192,
		}
		err := ValidateModelConfig(cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "model")
	})

	t.Run("zero max-model-len", func(t *testing.T) {
		cfg := &VLLMConfig{
			Model:       "Qwen/Qwen3-8B",
			MaxModelLen: 0,
		}
		err := ValidateModelConfig(cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "max-model-len")
	})

	t.Run("excessive max-model-len", func(t *testing.T) {
		cfg := &VLLMConfig{
			Model:       "Qwen/Qwen3-8B",
			MaxModelLen: 200000,
		}
		err := ValidateModelConfig(cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "max-model-len")
	})
}

func TestValidateGPUAllocation(t *testing.T) {
	t.Run("single GPU matches", func(t *testing.T) {
		err := ValidateGPUAllocation([]byte(validVLLMDeploymentYAML), 1)
		assert.NoError(t, err)
	})

	t.Run("expected 2 GPUs but only 1", func(t *testing.T) {
		err := ValidateGPUAllocation([]byte(validVLLMDeploymentYAML), 2)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected 2")
	})

	t.Run("no GPU allocation", func(t *testing.T) {
		yaml := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: vllm-cpu
spec:
  template:
    spec:
      containers:
        - name: vllm
          image: vllm/vllm-openai:latest
`
		err := ValidateGPUAllocation([]byte(yaml), 1)
		assert.Error(t, err)
	})

	t.Run("multi-GPU allocation", func(t *testing.T) {
		yaml := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: vllm-multi
spec:
  template:
    spec:
      containers:
        - name: vllm
          image: vllm/vllm-openai:latest
          resources:
            limits:
              nvidia.com/gpu: "2"
`
		err := ValidateGPUAllocation([]byte(yaml), 2)
		assert.NoError(t, err)
	})
}

func TestValidateDockerComposeVLLM(t *testing.T) {
	validCompose := `version: "3.8"
services:
  vllm-3090:
    image: vllm/vllm-openai:latest
    runtime: nvidia
    environment:
      - NVIDIA_VISIBLE_DEVICES=0
    command:
      - --model=Qwen/Qwen3-8B
      - --max-model-len=8192
    ports:
      - "8000:8000"
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8000/health"]
      interval: 30s
      timeout: 10s
      retries: 3
    volumes:
      - /mnt/models:/root/.cache/huggingface
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: 1
              capabilities: [gpu]
`
	t.Run("valid docker compose", func(t *testing.T) {
		result, err := ValidateDockerComposeVLLM([]byte(validCompose))
		require.NoError(t, err)
		assert.True(t, result.OK(), "valid compose should pass; failures: %v", result.Failures)
	})

	t.Run("missing runtime", func(t *testing.T) {
		yaml := `version: "3.8"
services:
  vllm-3090:
    image: vllm/vllm-openai:latest
    ports:
      - "8000:8000"
`
		result, err := ValidateDockerComposeVLLM([]byte(yaml))
		require.NoError(t, err)
		assert.False(t, result.OK())
	})
}
