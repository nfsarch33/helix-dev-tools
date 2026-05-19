package pplxbridge_test

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/pplxbridge"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeBase64Int8(t *testing.T) {
	raw := []int8{127, -128, 0, 64, -64}
	b := make([]byte, len(raw))
	for i, v := range raw {
		b[i] = byte(v)
	}
	encoded := base64.StdEncoding.EncodeToString(b)

	decoded, err := pplxbridge.DecodeBase64Int8(encoded)
	require.NoError(t, err)
	assert.Equal(t, raw, decoded)
}

func TestDecodeBase64Int8_Invalid(t *testing.T) {
	_, err := pplxbridge.DecodeBase64Int8("not-valid-base64!!!")
	assert.Error(t, err)
}

func TestInt8ToFloat32(t *testing.T) {
	input := []int8{127, -127, 0, 64}
	result := pplxbridge.Int8ToFloat32(input)
	assert.InDelta(t, 1.0, result[0], 0.01)
	assert.InDelta(t, -1.0, result[1], 0.01)
	assert.InDelta(t, 0.0, result[2], 0.01)
	assert.InDelta(t, 0.504, result[3], 0.01)
}

func TestZeroPad(t *testing.T) {
	vec := []float32{0.1, 0.2, 0.3}
	padded := pplxbridge.ZeroPad(vec, 6)
	assert.Len(t, padded, 6)
	assert.Equal(t, float32(0.1), padded[0])
	assert.Equal(t, float32(0.2), padded[1])
	assert.Equal(t, float32(0.3), padded[2])
	assert.Equal(t, float32(0), padded[3])
	assert.Equal(t, float32(0), padded[4])
	assert.Equal(t, float32(0), padded[5])
}

func TestZeroPad_AlreadyLarger(t *testing.T) {
	vec := []float32{0.1, 0.2, 0.3, 0.4, 0.5}
	padded := pplxbridge.ZeroPad(vec, 3)
	assert.Len(t, padded, 5)
}

func TestTranslateRequest(t *testing.T) {
	openaiReq := pplxbridge.OpenAIEmbedRequest{
		Input: []string{"hello world"},
		Model: "text-embedding-ada-002",
	}
	cfg := pplxbridge.Config{Model: "pplx-embed-v1-0.6b", Dimensions: 1024}
	pplxReq := pplxbridge.TranslateRequest(openaiReq, cfg)
	assert.Equal(t, "pplx-embed-v1-0.6b", pplxReq.Model)
	assert.Equal(t, []string{"hello world"}, pplxReq.Input)
	assert.Equal(t, 1024, pplxReq.Dimensions)
}

func TestTranslateResponse(t *testing.T) {
	raw := make([]int8, 4)
	raw[0], raw[1], raw[2], raw[3] = 127, -127, 64, 0
	b := make([]byte, len(raw))
	for i, v := range raw {
		b[i] = byte(v)
	}
	encoded := base64.StdEncoding.EncodeToString(b)

	pplxResp := pplxbridge.PerplexityResponse{
		Data: []pplxbridge.PerplexityEmbedding{
			{Embedding: encoded, Index: 0},
		},
		Model: "pplx-embed-v1-0.6b",
		Usage: pplxbridge.Usage{PromptTokens: 5, TotalTokens: 5},
	}

	cfg := pplxbridge.Config{OutputDimensions: 8}
	openaiResp := pplxbridge.TranslateResponse(pplxResp, cfg)
	assert.Equal(t, "list", openaiResp.Object)
	assert.Len(t, openaiResp.Data, 1)
	assert.Len(t, openaiResp.Data[0].Embedding, 8)
	assert.InDelta(t, 1.0, openaiResp.Data[0].Embedding[0], 0.01)
	assert.Equal(t, float32(0), openaiResp.Data[0].Embedding[7])
}

func TestBridge_Healthz(t *testing.T) {
	bridge := pplxbridge.NewBridge(pplxbridge.Config{})
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "ok")
}

func TestBridge_EmbeddingsEndpoint(t *testing.T) {
	mockPplx := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := make([]int8, 4)
		raw[0], raw[1], raw[2], raw[3] = 100, -50, 30, 0
		b := make([]byte, len(raw))
		for i, v := range raw {
			b[i] = byte(v)
		}
		encoded := base64.StdEncoding.EncodeToString(b)
		resp := pplxbridge.PerplexityResponse{
			Data:  []pplxbridge.PerplexityEmbedding{{Embedding: encoded, Index: 0}},
			Model: "pplx-embed-v1-0.6b",
			Usage: pplxbridge.Usage{PromptTokens: 3, TotalTokens: 3},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockPplx.Close()

	bridge := pplxbridge.NewBridge(pplxbridge.Config{
		APIKey:           "test-key",
		Model:            "pplx-embed-v1-0.6b",
		Dimensions:       4,
		OutputDimensions: 8,
		UpstreamURL:      mockPplx.URL,
	})

	body := `{"input":["test text"],"model":"text-embedding-ada-002"}`
	req := httptest.NewRequest("POST", "/v1/embeddings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	respBody, _ := io.ReadAll(w.Result().Body)
	var openaiResp pplxbridge.OpenAIEmbedResponse
	err := json.Unmarshal(respBody, &openaiResp)
	require.NoError(t, err)
	assert.Equal(t, "list", openaiResp.Object)
	assert.Len(t, openaiResp.Data, 1)
	assert.Len(t, openaiResp.Data[0].Embedding, 8)
}

func TestConfig_Defaults(t *testing.T) {
	cfg := pplxbridge.DefaultConfig()
	assert.Equal(t, "pplx-embed-v1-0.6b", cfg.Model)
	assert.Equal(t, 1024, cfg.Dimensions)
	assert.Equal(t, 1536, cfg.OutputDimensions)
	assert.Equal(t, ":8510", cfg.ListenAddr)
	assert.Equal(t, "https://api.perplexity.ai", cfg.UpstreamURL)
}
