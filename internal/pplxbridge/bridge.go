package pplxbridge

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// Config holds bridge configuration from environment variables.
type Config struct {
	APIKey           string
	Model            string
	Dimensions       int
	OutputDimensions int
	ListenAddr       string
	UpstreamURL      string
}

// DefaultConfig returns configuration with sensible defaults.
func DefaultConfig() Config {
	return Config{
		APIKey:           os.Getenv("PPLX_API_KEY"),
		Model:            envOr("PPLX_MODEL", "pplx-embed-v1-0.6b"),
		Dimensions:       envIntOr("PPLX_DIMENSIONS", 1024),
		OutputDimensions: envIntOr("OUTPUT_DIMENSIONS", 1536),
		ListenAddr:       envOr("LISTEN_ADDR", ":8510"),
		UpstreamURL:      envOr("PPLX_UPSTREAM_URL", "https://api.perplexity.ai"),
	}
}

// OpenAI-compatible request/response types

type OpenAIEmbedRequest struct {
	Input []string `json:"input"`
	Model string   `json:"model"`
}

func (r *OpenAIEmbedRequest) UnmarshalJSON(data []byte) error {
	type plain struct {
		Input json.RawMessage `json:"input"`
		Model string          `json:"model"`
	}
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	r.Model = p.Model
	if len(p.Input) == 0 {
		return nil
	}
	if p.Input[0] == '"' {
		var s string
		if err := json.Unmarshal(p.Input, &s); err != nil {
			return err
		}
		r.Input = []string{s}
		return nil
	}
	return json.Unmarshal(p.Input, &r.Input)
}

type OpenAIEmbedResponse struct {
	Object string            `json:"object"`
	Data   []OpenAIEmbedding `json:"data"`
	Model  string            `json:"model"`
	Usage  Usage             `json:"usage"`
}

type OpenAIEmbedding struct {
	Object    string    `json:"object"`
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}

// Perplexity-specific types

type PerplexityRequest struct {
	Input      []string `json:"input"`
	Model      string   `json:"model"`
	Dimensions int      `json:"dimensions,omitempty"`
}

type PerplexityResponse struct {
	Data  []PerplexityEmbedding `json:"data"`
	Model string                `json:"model"`
	Usage Usage                 `json:"usage"`
}

type PerplexityEmbedding struct {
	Embedding string `json:"embedding"`
	Index     int    `json:"index"`
}

type Usage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// DecodeBase64Int8 decodes a base64-encoded string to a signed int8 slice.
func DecodeBase64Int8(b64 string) ([]int8, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}
	result := make([]int8, len(raw))
	for i, b := range raw {
		result[i] = int8(b)
	}
	return result, nil
}

// Int8ToFloat32 normalizes int8 values to float32 in [-1, 1] range.
func Int8ToFloat32(vals []int8) []float32 {
	result := make([]float32, len(vals))
	for i, v := range vals {
		result[i] = float32(v) / 127.0
	}
	return result
}

// ZeroPad extends a vector to targetDim by appending zeros.
func ZeroPad(vec []float32, targetDim int) []float32 {
	if len(vec) >= targetDim {
		return vec
	}
	padded := make([]float32, targetDim)
	copy(padded, vec)
	return padded
}

// TranslateRequest converts an OpenAI-format request to Perplexity format.
func TranslateRequest(req OpenAIEmbedRequest, cfg Config) PerplexityRequest {
	return PerplexityRequest{
		Input:      req.Input,
		Model:      cfg.Model,
		Dimensions: cfg.Dimensions,
	}
}

// TranslateResponse converts a Perplexity response to OpenAI format.
func TranslateResponse(resp PerplexityResponse, cfg Config) OpenAIEmbedResponse {
	var embeddings []OpenAIEmbedding
	for _, e := range resp.Data {
		int8Vals, err := DecodeBase64Int8(e.Embedding)
		if err != nil {
			continue
		}
		floats := Int8ToFloat32(int8Vals)
		padded := ZeroPad(floats, cfg.OutputDimensions)
		embeddings = append(embeddings, OpenAIEmbedding{
			Object:    "embedding",
			Embedding: padded,
			Index:     e.Index,
		})
	}
	return OpenAIEmbedResponse{
		Object: "list",
		Data:   embeddings,
		Model:  resp.Model,
		Usage:  resp.Usage,
	}
}

// Bridge is the HTTP handler that proxies embedding requests.
type Bridge struct {
	config Config
	client *http.Client
}

// NewBridge creates a bridge with the given config.
func NewBridge(cfg Config) *Bridge {
	return &Bridge{config: cfg, client: &http.Client{}}
}

// ServeHTTP implements http.Handler.
func (b *Bridge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/healthz" && r.Method == "GET":
		w.WriteHeader(200)
		w.Write([]byte(`{"status":"ok"}`))
	case r.URL.Path == "/v1/embeddings" && r.Method == "POST":
		b.handleEmbeddings(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (b *Bridge) handleEmbeddings(w http.ResponseWriter, r *http.Request) {
	var openaiReq OpenAIEmbedRequest
	if err := json.NewDecoder(r.Body).Decode(&openaiReq); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, 400)
		return
	}

	pplxReq := TranslateRequest(openaiReq, b.config)
	reqBody, _ := json.Marshal(pplxReq)

	upstream := b.config.UpstreamURL + "/v1/embeddings"
	httpReq, _ := http.NewRequestWithContext(r.Context(), "POST", upstream, bytes.NewReader(reqBody))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+b.config.APIKey)

	resp, err := b.client.Do(httpReq)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"upstream: %s"}`, err.Error()), 502)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		w.WriteHeader(resp.StatusCode)
		w.Write(body)
		return
	}

	var pplxResp PerplexityResponse
	if err := json.Unmarshal(body, &pplxResp); err != nil {
		http.Error(w, `{"error":"failed to parse upstream response"}`, 502)
		return
	}

	openaiResp := TranslateResponse(pplxResp, b.config)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(openaiResp)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOr(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	var n int
	fmt.Sscanf(v, "%d", &n)
	if n == 0 {
		return fallback
	}
	return n
}
