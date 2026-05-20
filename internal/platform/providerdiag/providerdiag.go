package providerdiag

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

type ProviderType string

const (
	ProviderMiniMax    ProviderType = "minimax"
	ProviderPerplexity ProviderType = "perplexity"
	ProviderOpenAI     ProviderType = "openai"
	ProviderLocal      ProviderType = "local"
	ProviderUnknown    ProviderType = "unknown"
)

type Diagnosis struct {
	Provider     ProviderType `json:"provider"`
	BaseURL      string       `json:"base_url"`
	Model        string       `json:"model"`
	Issues       []Issue      `json:"issues"`
	Healthy      bool         `json:"healthy"`
	DiagnosedAt  time.Time    `json:"diagnosed_at"`
}

type Issue struct {
	Severity string `json:"severity"` // critical, warning, info
	Code     string `json:"code"`
	Message  string `json:"message"`
	Fix      string `json:"fix,omitempty"`
}

type EnvConfig struct {
	LLMBaseURL        string
	LLMModel          string
	LLMAPIKey         string
	EmbeddingBaseURL  string
	EmbeddingModel    string
	EmbeddingAPIKey   string
	Timeout           string
}

func DetectProvider(baseURL string) ProviderType {
	if baseURL == "" {
		return ProviderUnknown
	}
	lower := strings.ToLower(baseURL)
	switch {
	case strings.Contains(lower, "minimax") || strings.Contains(lower, "minimaxi.com"):
		return ProviderMiniMax
	case strings.Contains(lower, "perplexity") || strings.Contains(lower, "pplx"):
		return ProviderPerplexity
	case strings.Contains(lower, "openai.com"):
		return ProviderOpenAI
	case strings.Contains(lower, "127.0.0.1") || strings.Contains(lower, "localhost") || strings.Contains(lower, "host.docker.internal"):
		return ProviderLocal
	default:
		return ProviderUnknown
	}
}

func DiagnoseEnv(env EnvConfig) Diagnosis {
	d := Diagnosis{
		DiagnosedAt: time.Now(),
		Healthy:     true,
	}

	d.Provider = DetectProvider(env.LLMBaseURL)
	d.BaseURL = env.LLMBaseURL
	d.Model = env.LLMModel

	if env.LLMBaseURL == "" {
		d.addIssue("critical", "NO_LLM_URL", "LLM base URL not configured", "Set LLM_BASE_URL in Mem0 .env")
	}
	if env.EmbeddingBaseURL == "" {
		d.addIssue("critical", "NO_EMBED_URL", "Embedding base URL not configured", "Set OPENAI_EMBEDDING_BASE_URL in Mem0 .env")
	}

	if env.LLMAPIKey == "" && d.Provider != ProviderLocal {
		d.addIssue("critical", "NO_LLM_KEY", "LLM API key missing for remote provider", "Set LLM API key")
	}
	if env.EmbeddingAPIKey == "" && DetectProvider(env.EmbeddingBaseURL) != ProviderLocal {
		d.addIssue("warning", "NO_EMBED_KEY", "Embedding API key missing", "May be shared with LLM key")
	}

	if d.Provider == ProviderLocal && env.LLMModel != "" {
		if !strings.Contains(strings.ToLower(env.LLMModel), "minimax") &&
			!strings.Contains(strings.ToLower(env.LLMModel), "m2") {
			d.addIssue("warning", "LOCAL_LLM_NOT_MINIMAX",
				fmt.Sprintf("Local LLM model %q is not MiniMax M2.7-highspeed", env.LLMModel),
				"Switch to MiniMax-M2.7-highspeed API for production Mem0")
		}
	}

	if env.Timeout != "" {
		if timeout, err := time.ParseDuration(env.Timeout); err == nil {
			if timeout < 30*time.Second {
				d.addIssue("warning", "LOW_TIMEOUT",
					fmt.Sprintf("Timeout %v may be too low for 3-call infer chain", timeout),
					"Set timeout >= 30s for infer=true, or use infer=false")
			}
		}
	}

	if env.LLMBaseURL != "" {
		if _, err := url.Parse(env.LLMBaseURL); err != nil {
			d.addIssue("critical", "INVALID_LLM_URL", "Cannot parse LLM base URL: "+err.Error(), "Fix URL format")
		}
	}
	if env.EmbeddingBaseURL != "" {
		if _, err := url.Parse(env.EmbeddingBaseURL); err != nil {
			d.addIssue("critical", "INVALID_EMBED_URL", "Cannot parse embedding base URL: "+err.Error(), "Fix URL format")
		}
	}

	return d
}

func (d *Diagnosis) addIssue(severity, code, message, fix string) {
	d.Issues = append(d.Issues, Issue{
		Severity: severity,
		Code:     code,
		Message:  message,
		Fix:      fix,
	})
	if severity == "critical" {
		d.Healthy = false
	}
}

func (d Diagnosis) CriticalCount() int {
	count := 0
	for _, i := range d.Issues {
		if i.Severity == "critical" {
			count++
		}
	}
	return count
}

func (d Diagnosis) Summary() string {
	if d.Healthy {
		return fmt.Sprintf("OK: provider=%s model=%s (%d warnings)", d.Provider, d.Model, len(d.Issues))
	}
	return fmt.Sprintf("FAIL: %d critical issues, provider=%s", d.CriticalCount(), d.Provider)
}
