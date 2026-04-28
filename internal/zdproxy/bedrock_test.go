package zdproxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeBedrockUpstream stands in for the ZD Bedrock surface during unit tests.
// It records the most recent inbound request so tests can assert on URL path,
// method, headers, and body, and serves a configurable response (single-shot
// or streamed).
type fakeBedrockUpstream struct {
	server *httptest.Server

	mu          sync.Mutex
	lastPath    string
	lastMethod  string
	lastHeaders http.Header
	lastBody    []byte

	respStatus int
	respBody   []byte
	respHeader http.Header

	streamChunks [][]byte
	streamDelay  time.Duration
}

func newFakeBedrockUpstream() *fakeBedrockUpstream {
	f := &fakeBedrockUpstream{
		respStatus: http.StatusOK,
		respBody:   []byte(`{"id":"msg_test","type":"message","role":"assistant","content":[{"type":"text","text":"ok"}],"model":"claude-3-5-haiku","stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`),
		respHeader: http.Header{"Content-Type": []string{"application/json"}},
	}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.lastPath = r.URL.Path
		f.lastMethod = r.Method
		f.lastHeaders = r.Header.Clone()
		f.lastBody, _ = io.ReadAll(r.Body)
		chunks := f.streamChunks
		delay := f.streamDelay
		respHeader := f.respHeader
		respStatus := f.respStatus
		respBody := f.respBody
		f.mu.Unlock()

		for k, vv := range respHeader {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(respStatus)
		if len(chunks) > 0 {
			flusher, _ := w.(http.Flusher)
			for _, c := range chunks {
				_, _ = w.Write(c)
				if flusher != nil {
					flusher.Flush()
				}
				if delay > 0 {
					time.Sleep(delay)
				}
			}
			return
		}
		_, _ = w.Write(respBody)
	}))
	return f
}

func (f *fakeBedrockUpstream) close() {
	f.server.Close()
}

func (f *fakeBedrockUpstream) snapshot() (path, method string, headers http.Header, body []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastPath, f.lastMethod, f.lastHeaders, append([]byte(nil), f.lastBody...)
}

func newBedrockTransportForTest(upstreamURL, bearer string) *BedrockTransport {
	return &BedrockTransport{
		UpstreamBaseURL: upstreamURL,
		UpstreamBearer:  bearer,
		HTTPClient:      &http.Client{Timeout: 5 * time.Second},
	}
}

func TestBedrockTransport_AnthropicMessages_NonStreaming_ForwardsToInvoke(t *testing.T) {
	up := newFakeBedrockUpstream()
	defer up.close()

	bt := newBedrockTransportForTest(up.server.URL, "BEDROCK_BEARER")

	body := `{"model":"us.anthropic.claude-opus-4-7","messages":[{"role":"user","content":"hi"}],"max_tokens":8}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	bt.HandleAnthropicMessages(rec, req)

	path, method, headers, _ := up.snapshot()
	if path != "/model/us.anthropic.claude-opus-4-7/invoke" {
		t.Fatalf("expected upstream path /model/us.anthropic.claude-opus-4-7/invoke, got %q", path)
	}
	if method != http.MethodPost {
		t.Fatalf("expected upstream POST, got %q", method)
	}
	if got := headers.Get("Authorization"); got != "Bearer BEDROCK_BEARER" {
		t.Fatalf("expected Authorization=Bearer BEDROCK_BEARER, got %q", got)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%q", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"type":"message"`)) {
		t.Fatalf("expected upstream body forwarded, got %q", rec.Body.String())
	}
}

func TestBedrockTransport_AnthropicMessages_InjectsAnthropicVersion(t *testing.T) {
	up := newFakeBedrockUpstream()
	defer up.close()

	bt := newBedrockTransportForTest(up.server.URL, "X")
	body := `{"model":"us.anthropic.claude-3-5-haiku-20241022-v1:0","messages":[],"max_tokens":4}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	bt.HandleAnthropicMessages(httptest.NewRecorder(), req)

	_, _, _, upBody := up.snapshot()
	var got map[string]any
	if err := json.Unmarshal(upBody, &got); err != nil {
		t.Fatalf("upstream body not JSON: %v (body=%q)", err, upBody)
	}
	if got["anthropic_version"] != "bedrock-2023-05-31" {
		t.Fatalf("expected anthropic_version injected as bedrock-2023-05-31, got %v", got["anthropic_version"])
	}
}

func TestBedrockTransport_AnthropicMessages_StripsModelAndStreamFromBody(t *testing.T) {
	// Bedrock invoke rejects bodies that include `model` (it lives in the
	// URL path). It also rejects `stream` on the non-streaming endpoint.
	// The prepare step strips both so callers can use the Anthropic-Messages
	// shape transparently.
	up := newFakeBedrockUpstream()
	defer up.close()

	bt := newBedrockTransportForTest(up.server.URL, "X")
	body := `{"model":"us.anthropic.claude-opus-4-7","stream":false,"messages":[{"role":"user","content":"hi"}],"max_tokens":4}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	bt.HandleAnthropicMessages(httptest.NewRecorder(), req)

	_, _, _, upBody := up.snapshot()
	var got map[string]any
	if err := json.Unmarshal(upBody, &got); err != nil {
		t.Fatalf("upstream body not JSON: %v", err)
	}
	if _, ok := got["model"]; ok {
		t.Fatalf("expected `model` stripped from upstream body, got %v", got)
	}
	if _, ok := got["stream"]; ok {
		t.Fatalf("expected `stream` stripped from upstream body, got %v", got)
	}
	if got["anthropic_version"] != "bedrock-2023-05-31" {
		t.Fatalf("expected anthropic_version injected, got %v", got["anthropic_version"])
	}
}

func TestBedrockTransport_AnthropicMessages_PreservesAnthropicVersion(t *testing.T) {
	up := newFakeBedrockUpstream()
	defer up.close()

	bt := newBedrockTransportForTest(up.server.URL, "X")
	body := `{"anthropic_version":"bedrock-2023-05-31","model":"us.anthropic.claude-opus-4-7","messages":[],"max_tokens":4}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	bt.HandleAnthropicMessages(httptest.NewRecorder(), req)

	_, _, _, upBody := up.snapshot()
	var got map[string]any
	if err := json.Unmarshal(upBody, &got); err != nil {
		t.Fatalf("upstream body not JSON: %v", err)
	}
	if got["anthropic_version"] != "bedrock-2023-05-31" {
		t.Fatalf("expected anthropic_version preserved, got %v", got["anthropic_version"])
	}
}

func TestBedrockTransport_AnthropicMessages_StreamingUsesInvokeWithResponseStream(t *testing.T) {
	up := newFakeBedrockUpstream()
	up.streamChunks = [][]byte{[]byte("chunk1"), []byte("chunk2"), []byte("chunk3")}
	up.respHeader = http.Header{"Content-Type": []string{"application/vnd.amazon.eventstream"}}
	defer up.close()

	bt := newBedrockTransportForTest(up.server.URL, "X")
	body := `{"model":"us.anthropic.claude-opus-4-7","messages":[],"max_tokens":4,"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	rec := httptest.NewRecorder()
	bt.HandleAnthropicMessages(rec, req)

	path, _, _, _ := up.snapshot()
	if path != "/model/us.anthropic.claude-opus-4-7/invoke-with-response-stream" {
		t.Fatalf("expected streaming endpoint, got %q", path)
	}
	if rec.Header().Get("Content-Type") != "application/vnd.amazon.eventstream" {
		t.Fatalf("expected event-stream Content-Type, got %q", rec.Header().Get("Content-Type"))
	}
	if got := rec.Body.String(); got != "chunk1chunk2chunk3" {
		t.Fatalf("expected concatenated chunks, got %q", got)
	}
}

func TestBedrockTransport_AnthropicMessages_RejectsMissingModel(t *testing.T) {
	up := newFakeBedrockUpstream()
	defer up.close()

	bt := newBedrockTransportForTest(up.server.URL, "X")
	body := `{"messages":[],"max_tokens":4}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	rec := httptest.NewRecorder()
	bt.HandleAnthropicMessages(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing model, got %d", rec.Code)
	}
	path, _, _, _ := up.snapshot()
	if path != "" {
		t.Fatalf("upstream should not have been called, got path=%q", path)
	}
}

func TestBedrockTransport_AnthropicMessages_RejectsNonJSON(t *testing.T) {
	up := newFakeBedrockUpstream()
	defer up.close()

	bt := newBedrockTransportForTest(up.server.URL, "X")
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader("not json"))
	rec := httptest.NewRecorder()
	bt.HandleAnthropicMessages(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on non-JSON, got %d", rec.Code)
	}
}

func TestBedrockTransport_BedrockPassthrough_NonStreaming(t *testing.T) {
	up := newFakeBedrockUpstream()
	defer up.close()

	bt := newBedrockTransportForTest(up.server.URL, "X")
	body := `{"anthropic_version":"bedrock-2023-05-31","messages":[],"max_tokens":4}`
	req := httptest.NewRequest(http.MethodPost, "/bedrock/model/us.anthropic.claude-sonnet-4-6/invoke", strings.NewReader(body))
	rec := httptest.NewRecorder()
	bt.HandleBedrockPassthrough(rec, req)

	path, _, _, upBody := up.snapshot()
	if path != "/model/us.anthropic.claude-sonnet-4-6/invoke" {
		t.Fatalf("expected /model/.../invoke, got %q", path)
	}
	if !bytes.Equal(upBody, []byte(body)) {
		t.Fatalf("expected body forwarded byte-for-byte\nwant: %q\ngot:  %q", body, upBody)
	}
}

func TestBedrockTransport_BedrockPassthrough_Streaming(t *testing.T) {
	up := newFakeBedrockUpstream()
	up.streamChunks = [][]byte{[]byte("a"), []byte("b")}
	up.respHeader = http.Header{"Content-Type": []string{"application/vnd.amazon.eventstream"}}
	defer up.close()

	bt := newBedrockTransportForTest(up.server.URL, "X")
	body := `{"messages":[],"max_tokens":4}`
	req := httptest.NewRequest(http.MethodPost, "/bedrock/model/us.anthropic.claude-opus-4-7/invoke-with-response-stream", strings.NewReader(body))
	rec := httptest.NewRecorder()
	bt.HandleBedrockPassthrough(rec, req)

	path, _, _, _ := up.snapshot()
	if path != "/model/us.anthropic.claude-opus-4-7/invoke-with-response-stream" {
		t.Fatalf("expected streaming path, got %q", path)
	}
	if rec.Body.String() != "ab" {
		t.Fatalf("expected concatenated stream chunks, got %q", rec.Body.String())
	}
}

func TestBedrockTransport_BedrockPassthrough_RejectsInvalidPath(t *testing.T) {
	bt := newBedrockTransportForTest("http://upstream.invalid", "X")

	cases := []struct {
		name string
		path string
	}{
		{"missing-model-segment", "/bedrock/wrong/foo/invoke"},
		{"missing-endpoint", "/bedrock/model/foo"},
		{"unknown-endpoint", "/bedrock/model/foo/list"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader("{}"))
			rec := httptest.NewRecorder()
			bt.HandleBedrockPassthrough(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 on %q, got %d (body=%q)", tc.path, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestBedrockTransport_PropagatesUpstreamStatus(t *testing.T) {
	up := newFakeBedrockUpstream()
	up.respStatus = http.StatusBadRequest
	up.respBody = []byte(`{"error":{"type":"invalid_request_error","message":"bad model"}}`)
	defer up.close()

	bt := newBedrockTransportForTest(up.server.URL, "X")
	body := `{"model":"x","messages":[],"max_tokens":4}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	rec := httptest.NewRecorder()
	bt.HandleAnthropicMessages(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 propagated, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "invalid_request_error") {
		t.Fatalf("expected upstream body forwarded, got %q", rec.Body.String())
	}
}

func TestBedrockTransport_DoesNotLeakBearerOnUpstreamConnError(t *testing.T) {
	up := newFakeBedrockUpstream()
	upURL := up.server.URL
	up.close() // server closed → connection refused

	bearer := "SUPER_SECRET_TOKEN_VALUE_THAT_MUST_NEVER_LEAK"
	bt := newBedrockTransportForTest(upURL, bearer)
	body := `{"model":"x","messages":[],"max_tokens":4}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	rec := httptest.NewRecorder()
	bt.HandleAnthropicMessages(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 on upstream conn error, got %d (body=%q)", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), bearer) {
		t.Fatalf("response must not contain upstream bearer, got %q", rec.Body.String())
	}
}

func TestBedrockTransport_RejectsNonPostMethod(t *testing.T) {
	bt := newBedrockTransportForTest("http://upstream.invalid", "X")

	req := httptest.NewRequest(http.MethodGet, "/v1/messages", nil)
	rec := httptest.NewRecorder()
	bt.HandleAnthropicMessages(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 on Anthropic GET, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/bedrock/model/x/invoke", nil)
	rec = httptest.NewRecorder()
	bt.HandleBedrockPassthrough(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 on Bedrock GET, got %d", rec.Code)
	}
}

func TestBedrockTransport_StreamingFlushesIncrementally(t *testing.T) {
	up := newFakeBedrockUpstream()
	up.streamChunks = [][]byte{[]byte("chunk-A"), []byte("chunk-B")}
	up.respHeader = http.Header{"Content-Type": []string{"application/vnd.amazon.eventstream"}}
	up.streamDelay = 30 * time.Millisecond
	defer up.close()

	bt := newBedrockTransportForTest(up.server.URL, "X")

	proxyMux := http.NewServeMux()
	proxyMux.HandleFunc("/v1/messages", bt.HandleAnthropicMessages)
	proxy := httptest.NewServer(proxyMux)
	defer proxy.Close()

	body := `{"model":"x","messages":[],"max_tokens":1,"stream":true}`
	resp, err := http.Post(proxy.URL+"/v1/messages", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Content-Type"); got != "application/vnd.amazon.eventstream" {
		t.Fatalf("expected event-stream Content-Type, got %q", got)
	}
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(out) != "chunk-Achunk-B" {
		t.Fatalf("expected concatenated chunks, got %q", string(out))
	}
}
