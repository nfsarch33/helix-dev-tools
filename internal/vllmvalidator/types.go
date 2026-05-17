package vllmvalidator

// VLLMConfig holds the vLLM serving configuration parameters.
type VLLMConfig struct {
	Model        string `yaml:"model" json:"model"`
	Quantization string `yaml:"quantization" json:"quantization"`
	MaxModelLen  int    `yaml:"max_model_len" json:"max_model_len"`
	GPUMemUtil   float64 `yaml:"gpu_memory_utilization" json:"gpu_memory_utilization"`
}

// ValidationResult collects pass/fail findings from a vLLM manifest check.
type ValidationResult struct {
	Name     string
	Passed   []string
	Failures []string
}

func (v *ValidationResult) OK() bool { return len(v.Failures) == 0 }
