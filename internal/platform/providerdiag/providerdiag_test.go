package providerdiag

import (
	"testing"
)

func TestDetectProvider(t *testing.T) {
	tests := []struct {
		url  string
		want ProviderType
	}{
		{"https://api.minimaxi.com/v1", ProviderMiniMax},
		{"http://127.0.0.1:8787/v1", ProviderLocal},
		{"http://localhost:8000/v1", ProviderLocal},
		{"http://host.docker.internal:8510/v1", ProviderLocal},
		{"https://api.openai.com/v1", ProviderOpenAI},
		{"https://api.perplexity.ai/v1", ProviderPerplexity},
		{"https://random-provider.com/v1", ProviderUnknown},
		{"", ProviderUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := DetectProvider(tt.url)
			if got != tt.want {
				t.Errorf("DetectProvider(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestDiagnoseEnvHealthy(t *testing.T) {
	d := DiagnoseEnv(EnvConfig{
		LLMBaseURL:       "https://api.minimaxi.com/v1",
		LLMModel:         "MiniMax-M2.7-highspeed",
		LLMAPIKey:        "set",
		EmbeddingBaseURL: "http://host.docker.internal:8510/v1",
		EmbeddingModel:   "pplx-embed-v1",
		Timeout:          "90s",
	})
	if !d.Healthy {
		t.Fatalf("expected healthy, got issues: %v", d.Issues)
	}
	if d.Provider != ProviderMiniMax {
		t.Errorf("expected minimax, got %v", d.Provider)
	}
}

func TestDiagnoseEnvMissingURLs(t *testing.T) {
	d := DiagnoseEnv(EnvConfig{})
	if d.Healthy {
		t.Fatal("should not be healthy with empty config")
	}
	if d.CriticalCount() < 2 {
		t.Errorf("expected at least 2 critical issues, got %d", d.CriticalCount())
	}
}

func TestDiagnoseEnvLocalNotMiniMax(t *testing.T) {
	d := DiagnoseEnv(EnvConfig{
		LLMBaseURL:       "http://127.0.0.1:8787/v1",
		LLMModel:         "Qwen3.5-4B",
		EmbeddingBaseURL: "http://127.0.0.1:8510/v1",
	})
	found := false
	for _, issue := range d.Issues {
		if issue.Code == "LOCAL_LLM_NOT_MINIMAX" {
			found = true
		}
	}
	if !found {
		t.Error("expected LOCAL_LLM_NOT_MINIMAX warning")
	}
}

func TestDiagnoseEnvLowTimeout(t *testing.T) {
	d := DiagnoseEnv(EnvConfig{
		LLMBaseURL:       "https://api.minimaxi.com/v1",
		LLMAPIKey:        "set",
		EmbeddingBaseURL: "http://host.docker.internal:8510/v1",
		Timeout:          "10s",
	})
	found := false
	for _, issue := range d.Issues {
		if issue.Code == "LOW_TIMEOUT" {
			found = true
		}
	}
	if !found {
		t.Error("expected LOW_TIMEOUT warning for 10s")
	}
}

func TestDiagnoseEnvMissingAPIKey(t *testing.T) {
	d := DiagnoseEnv(EnvConfig{
		LLMBaseURL:       "https://api.minimaxi.com/v1",
		LLMModel:         "MiniMax-M2.7-highspeed",
		EmbeddingBaseURL: "http://host.docker.internal:8510/v1",
	})
	if d.Healthy {
		t.Fatal("should fail without API key for remote provider")
	}
}

func TestSummaryFormat(t *testing.T) {
	healthy := Diagnosis{Healthy: true, Provider: ProviderMiniMax, Model: "M2.7"}
	if s := healthy.Summary(); s == "" {
		t.Error("summary should not be empty")
	}

	unhealthy := Diagnosis{
		Healthy: false, Provider: ProviderUnknown,
		Issues: []Issue{{Severity: "critical", Code: "X"}},
	}
	if s := unhealthy.Summary(); s == "" {
		t.Error("summary should not be empty")
	}
}

func TestCriticalCount(t *testing.T) {
	d := Diagnosis{Issues: []Issue{
		{Severity: "critical"},
		{Severity: "warning"},
		{Severity: "critical"},
		{Severity: "info"},
	}}
	if d.CriticalCount() != 2 {
		t.Errorf("expected 2 critical, got %d", d.CriticalCount())
	}
}
