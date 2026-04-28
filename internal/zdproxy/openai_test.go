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

// fakeOpenAIUpstream stands in for the ZD OpenAI Chat Completions / Responses
// surface during unit tests. It records the most recent inbound request so
// tests can assert on URL path, method, headers, and body, and serves a
// configurable response (single-shot or streamed).
type fakeOpenAIUpstream struct {
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

func newFakeOpenAIUpstream() *fakeOpenAIUpstream {
	f := &fakeOpenAIUpstream{
		respStatus: http.StatusOK,
		respBody:   []byte(`{"id":"chatcmpl_test","object":"chat.completion","model":"gpt-5.5","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`),
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

func (f *fakeOpenAIUpstream) close() {
	f.server.Close()
}

func (f *fakeOpenAIUpstream) snapshot() (path, method string, headers http.Header, body []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastPath, f.lastMethod, f.lastHeaders, append([]byte(nil), f.lastBody...)
}

func newOpenAITransportForTest(upstreamURL, bearer string) *OpenAITransport {
	return &OpenAITransport{
		UpstreamBaseURL: upstreamURL,
		UpstreamBearer:  bearer,
		HTTPClient:      &http.Client{Timeout: 5 * time.Second},
	}
}

// --------------------------------------------------------------------------
// Chat Completions passthrough (Cursor → /v1/chat/completions)
// --------------------------------------------------------------------------

func TestOpenAITransport_ChatCompletions_Passthrough_NonStreaming(t *testing.T) {
	up := newFakeOpenAIUpstream()
	defer up.close()

	ot := newOpenAITransportForTest(up.server.URL, "OPENAI_BEARER")
	body := `{"model":"gpt-5.5","messages":[{"role":"user","content":"hi"}],"max_completion_tokens":8}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	ot.HandleChatCompletions(rec, req)

	path, method, headers, upBody := up.snapshot()
	if path != "/chat/completions" {
		t.Fatalf("expected upstream path /chat/completions, got %q", path)
	}
	if method != http.MethodPost {
		t.Fatalf("expected upstream POST, got %q", method)
	}
	if got := headers.Get("Authorization"); got != "Bearer OPENAI_BEARER" {
		t.Fatalf("expected Authorization=Bearer OPENAI_BEARER, got %q", got)
	}
	if !bytes.Equal(upBody, []byte(body)) {
		t.Fatalf("expected body forwarded byte-for-byte\nwant: %q\ngot:  %q", body, upBody)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"object":"chat.completion"`) {
		t.Fatalf("expected upstream body forwarded, got %q", rec.Body.String())
	}
}

func TestOpenAITransport_ChatCompletions_Passthrough_Streaming(t *testing.T) {
	up := newFakeOpenAIUpstream()
	up.streamChunks = [][]byte{
		[]byte("data: {\"id\":\"x\",\"choices\":[{\"delta\":{\"content\":\"hel\"}}]}\n\n"),
		[]byte("data: {\"id\":\"x\",\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}\n\n"),
		[]byte("data: [DONE]\n\n"),
	}
	up.respHeader = http.Header{"Content-Type": []string{"text/event-stream"}}
	defer up.close()

	ot := newOpenAITransportForTest(up.server.URL, "X")
	body := `{"model":"gpt-5.5","messages":[{"role":"user","content":"hi"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	ot.HandleChatCompletions(rec, req)

	path, _, _, _ := up.snapshot()
	if path != "/chat/completions" {
		t.Fatalf("expected /chat/completions, got %q", path)
	}
	if rec.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("expected text/event-stream, got %q", rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Body.String(), `"hel"`) || !strings.Contains(rec.Body.String(), `[DONE]`) {
		t.Fatalf("expected streamed SSE chunks, got %q", rec.Body.String())
	}
}

func TestOpenAITransport_ChatCompletions_RejectsNonPost(t *testing.T) {
	ot := newOpenAITransportForTest("http://upstream.invalid", "X")
	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	rec := httptest.NewRecorder()
	ot.HandleChatCompletions(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 on GET, got %d", rec.Code)
	}
}

func TestOpenAITransport_ChatCompletions_PropagatesUpstreamStatus(t *testing.T) {
	up := newFakeOpenAIUpstream()
	up.respStatus = http.StatusBadRequest
	up.respBody = []byte(`{"error":{"message":"bad model","type":"invalid_request_error"}}`)
	defer up.close()

	ot := newOpenAITransportForTest(up.server.URL, "X")
	body := `{"model":"nope","messages":[]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	ot.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 propagated, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "invalid_request_error") {
		t.Fatalf("expected upstream body forwarded, got %q", rec.Body.String())
	}
}

func TestOpenAITransport_ChatCompletions_DoesNotLeakBearer(t *testing.T) {
	up := newFakeOpenAIUpstream()
	upURL := up.server.URL
	up.close() // simulate refused connection

	bearer := "OPENAI_SUPER_SECRET_VALUE_THAT_MUST_NEVER_LEAK"
	ot := newOpenAITransportForTest(upURL, bearer)
	body := `{"model":"gpt-5.5","messages":[]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	ot.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 on upstream conn error, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), bearer) {
		t.Fatalf("response must not contain upstream bearer, got %q", rec.Body.String())
	}
}

// --------------------------------------------------------------------------
// Responses passthrough (Cursor → /v1/responses for o3/o4/codex)
// --------------------------------------------------------------------------

func TestOpenAITransport_Responses_Passthrough_NonStreaming(t *testing.T) {
	up := newFakeOpenAIUpstream()
	up.respBody = []byte(`{"id":"resp_test","object":"response","model":"gpt-5-codex","status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"ok"}]}]}`)
	defer up.close()

	ot := newOpenAITransportForTest(up.server.URL, "X")
	body := `{"model":"gpt-5-codex","input":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	rec := httptest.NewRecorder()
	ot.HandleResponses(rec, req)

	path, _, headers, upBody := up.snapshot()
	if path != "/responses" {
		t.Fatalf("expected /responses, got %q", path)
	}
	if got := headers.Get("Authorization"); got != "Bearer X" {
		t.Fatalf("expected Authorization=Bearer X, got %q", got)
	}
	if !bytes.Equal(upBody, []byte(body)) {
		t.Fatalf("expected body forwarded byte-for-byte\nwant: %q\ngot:  %q", body, upBody)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%q", rec.Code, rec.Body.String())
	}
}

func TestOpenAITransport_Responses_Passthrough_Streaming(t *testing.T) {
	up := newFakeOpenAIUpstream()
	up.streamChunks = [][]byte{
		[]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"hel\"}\n\n"),
		[]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"lo\"}\n\n"),
		[]byte("data: {\"type\":\"response.completed\"}\n\n"),
	}
	up.respHeader = http.Header{"Content-Type": []string{"text/event-stream"}}
	defer up.close()

	ot := newOpenAITransportForTest(up.server.URL, "X")
	body := `{"model":"gpt-5-codex","stream":true,"input":[]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	rec := httptest.NewRecorder()
	ot.HandleResponses(rec, req)

	if rec.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("expected text/event-stream, got %q", rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Body.String(), "response.output_text.delta") || !strings.Contains(rec.Body.String(), "response.completed") {
		t.Fatalf("expected streamed Responses SSE events, got %q", rec.Body.String())
	}
}

func TestOpenAITransport_Responses_RejectsNonPost(t *testing.T) {
	ot := newOpenAITransportForTest("http://upstream.invalid", "X")
	req := httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
	rec := httptest.NewRecorder()
	ot.HandleResponses(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 on GET, got %d", rec.Code)
	}
}

// --------------------------------------------------------------------------
// Pure translator: Anthropic Messages → OpenAI Chat Completions
// --------------------------------------------------------------------------

func TestTranslateAnthropicToOpenAIChat_BasicMessages(t *testing.T) {
	in := []byte(`{
		"model":"gpt-5.5",
		"max_tokens":128,
		"messages":[{"role":"user","content":"hi"}]
	}`)
	out, err := translateAnthropicToOpenAIChat(in)
	if err != nil {
		t.Fatalf("translateAnthropicToOpenAIChat: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("output not JSON: %v (out=%q)", err, out)
	}
	if got["model"] != "gpt-5.5" {
		t.Fatalf("expected model preserved, got %v", got["model"])
	}
	// max_tokens MUST be renamed to max_completion_tokens (gpt-5 family rejects max_tokens)
	if _, ok := got["max_tokens"]; ok {
		t.Fatalf("expected max_tokens removed, got %v", got)
	}
	if v, ok := got["max_completion_tokens"]; !ok || int(v.(float64)) != 128 {
		t.Fatalf("expected max_completion_tokens=128, got %v", got["max_completion_tokens"])
	}
	msgs, ok := got["messages"].([]any)
	if !ok || len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %v", got["messages"])
	}
	first := msgs[0].(map[string]any)
	if first["role"] != "user" || first["content"] != "hi" {
		t.Fatalf("expected user/hi, got %v", first)
	}
}

func TestTranslateAnthropicToOpenAIChat_SystemFieldFlattened(t *testing.T) {
	in := []byte(`{
		"model":"gpt-5.5",
		"max_tokens":8,
		"system":"You are concise.",
		"messages":[{"role":"user","content":"hi"}]
	}`)
	out, err := translateAnthropicToOpenAIChat(in)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	var got map[string]any
	_ = json.Unmarshal(out, &got)
	if _, ok := got["system"]; ok {
		t.Fatalf("expected top-level system removed, got %v", got)
	}
	msgs := got["messages"].([]any)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages (system+user), got %d (%v)", len(msgs), msgs)
	}
	sys := msgs[0].(map[string]any)
	if sys["role"] != "system" || sys["content"] != "You are concise." {
		t.Fatalf("expected first message to be system, got %v", sys)
	}
}

func TestTranslateAnthropicToOpenAIChat_SystemAsContentBlocks(t *testing.T) {
	// Anthropic also accepts system as an array of content blocks; we
	// concatenate text blocks and ignore non-text blocks (best-effort).
	in := []byte(`{
		"model":"gpt-5.5",
		"max_tokens":8,
		"system":[{"type":"text","text":"You are A."},{"type":"text","text":"You are B."}],
		"messages":[{"role":"user","content":"hi"}]
	}`)
	out, err := translateAnthropicToOpenAIChat(in)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	var got map[string]any
	_ = json.Unmarshal(out, &got)
	msgs := got["messages"].([]any)
	sys := msgs[0].(map[string]any)
	if sys["role"] != "system" {
		t.Fatalf("expected system role, got %v", sys)
	}
	if !strings.Contains(sys["content"].(string), "You are A.") || !strings.Contains(sys["content"].(string), "You are B.") {
		t.Fatalf("expected concatenated system text, got %v", sys["content"])
	}
}

func TestTranslateAnthropicToOpenAIChat_StopSequencesRenamed(t *testing.T) {
	in := []byte(`{
		"model":"gpt-5.5",
		"max_tokens":8,
		"messages":[{"role":"user","content":"hi"}],
		"stop_sequences":["\n\n","END"]
	}`)
	out, err := translateAnthropicToOpenAIChat(in)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	var got map[string]any
	_ = json.Unmarshal(out, &got)
	if _, ok := got["stop_sequences"]; ok {
		t.Fatalf("expected stop_sequences removed")
	}
	stop, ok := got["stop"].([]any)
	if !ok || len(stop) != 2 {
		t.Fatalf("expected stop=[\\n\\n,END], got %v", got["stop"])
	}
	if stop[1] != "END" {
		t.Fatalf("expected END preserved, got %v", stop[1])
	}
}

func TestTranslateAnthropicToOpenAIChat_ToolsTranslated(t *testing.T) {
	in := []byte(`{
		"model":"gpt-5.5",
		"max_tokens":8,
		"messages":[{"role":"user","content":"weather?"}],
		"tools":[
			{
				"name":"get_weather",
				"description":"Get the current weather.",
				"input_schema":{
					"type":"object",
					"properties":{"city":{"type":"string"}},
					"required":["city"]
				}
			}
		]
	}`)
	out, err := translateAnthropicToOpenAIChat(in)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	var got map[string]any
	_ = json.Unmarshal(out, &got)
	tools, ok := got["tools"].([]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("expected 1 translated tool, got %v", got["tools"])
	}
	t0 := tools[0].(map[string]any)
	if t0["type"] != "function" {
		t.Fatalf("expected type=function, got %v", t0)
	}
	fn, ok := t0["function"].(map[string]any)
	if !ok {
		t.Fatalf("expected function block, got %v", t0)
	}
	if fn["name"] != "get_weather" || fn["description"] != "Get the current weather." {
		t.Fatalf("expected function metadata preserved, got %v", fn)
	}
	params, ok := fn["parameters"].(map[string]any)
	if !ok || params["type"] != "object" {
		t.Fatalf("expected input_schema → parameters, got %v", fn["parameters"])
	}
}

func TestTranslateAnthropicToOpenAIChat_ToolUseBlockBecomesToolCall(t *testing.T) {
	// Anthropic assistant message with content blocks containing tool_use →
	// OpenAI assistant message with tool_calls list.
	in := []byte(`{
		"model":"gpt-5.5",
		"max_tokens":8,
		"messages":[
			{"role":"user","content":"weather?"},
			{
				"role":"assistant",
				"content":[
					{"type":"text","text":"Let me check."},
					{"type":"tool_use","id":"tu_1","name":"get_weather","input":{"city":"NYC"}}
				]
			}
		]
	}`)
	out, err := translateAnthropicToOpenAIChat(in)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	var got map[string]any
	_ = json.Unmarshal(out, &got)
	msgs := got["messages"].([]any)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	asst := msgs[1].(map[string]any)
	if asst["role"] != "assistant" {
		t.Fatalf("expected assistant role, got %v", asst)
	}
	if c, ok := asst["content"].(string); !ok || !strings.Contains(c, "Let me check.") {
		t.Fatalf("expected assistant text content preserved, got %v", asst["content"])
	}
	tc, ok := asst["tool_calls"].([]any)
	if !ok || len(tc) != 1 {
		t.Fatalf("expected tool_calls=[...] from tool_use block, got %v", asst["tool_calls"])
	}
	tc0 := tc[0].(map[string]any)
	if tc0["id"] != "tu_1" || tc0["type"] != "function" {
		t.Fatalf("expected id=tu_1 type=function, got %v", tc0)
	}
	fn := tc0["function"].(map[string]any)
	if fn["name"] != "get_weather" {
		t.Fatalf("expected function name=get_weather, got %v", fn)
	}
	// arguments must be a JSON-encoded string (OpenAI shape) not an object
	if _, ok := fn["arguments"].(string); !ok {
		t.Fatalf("expected arguments to be JSON string, got %T (%v)", fn["arguments"], fn["arguments"])
	}
}

func TestTranslateAnthropicToOpenAIChat_ToolResultBlockBecomesToolMessage(t *testing.T) {
	// Anthropic user message containing tool_result content block →
	// OpenAI message with role=tool and tool_call_id.
	in := []byte(`{
		"model":"gpt-5.5",
		"max_tokens":8,
		"messages":[
			{
				"role":"user",
				"content":[
					{"type":"tool_result","tool_use_id":"tu_1","content":"sunny, 72F"}
				]
			}
		]
	}`)
	out, err := translateAnthropicToOpenAIChat(in)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	var got map[string]any
	_ = json.Unmarshal(out, &got)
	msgs := got["messages"].([]any)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	tm := msgs[0].(map[string]any)
	if tm["role"] != "tool" {
		t.Fatalf("expected role=tool, got %v", tm)
	}
	if tm["tool_call_id"] != "tu_1" {
		t.Fatalf("expected tool_call_id=tu_1, got %v", tm["tool_call_id"])
	}
	if !strings.Contains(tm["content"].(string), "sunny") {
		t.Fatalf("expected content preserved, got %v", tm["content"])
	}
}

func TestTranslateAnthropicToOpenAIChat_StripsAnthropicVersion(t *testing.T) {
	// OpenAI Chat Completions rejects unknown top-level keys; anthropic_version
	// is not a thing on OpenAI.
	in := []byte(`{
		"anthropic_version":"bedrock-2023-05-31",
		"model":"gpt-5.5",
		"max_tokens":8,
		"messages":[{"role":"user","content":"hi"}]
	}`)
	out, err := translateAnthropicToOpenAIChat(in)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	var got map[string]any
	_ = json.Unmarshal(out, &got)
	if _, ok := got["anthropic_version"]; ok {
		t.Fatalf("expected anthropic_version removed for OpenAI, got %v", got)
	}
}

// --------------------------------------------------------------------------
// Model-based dispatch from /v1/messages
// --------------------------------------------------------------------------

func TestRouteAnthropicModel_BedrockShapes(t *testing.T) {
	for _, m := range []string{
		"claude-sonnet-4-6",
		"us.anthropic.claude-opus-4-7",
		"us.anthropic.claude-3-5-haiku-20241022-v1:0",
		"anthropic.claude-3-haiku-20240307-v1:0",
	} {
		got := routeAnthropicModel(m)
		if got != routeBedrockInvoke {
			t.Errorf("model %q: expected routeBedrockInvoke, got %v", m, got)
		}
	}
}

func TestRouteAnthropicModel_OpenAIChatShapes(t *testing.T) {
	for _, m := range []string{
		"gpt-5.5",
		"gpt-5.4",
		"gpt-5.4-mini",
		"gpt-4.1",
		"gpt-4o",
		"gpt-4o-mini",
	} {
		got := routeAnthropicModel(m)
		if got != routeOpenAIChat {
			t.Errorf("model %q: expected routeOpenAIChat, got %v", m, got)
		}
	}
}

func TestRouteAnthropicModel_OpenAIResponsesShapes(t *testing.T) {
	for _, m := range []string{
		"gpt-5-codex",
		"gpt-5.5-pro",
		"o3",
		"o3-mini",
		"o3-deep-research",
		"o4-mini",
		"o4-mini-deep-research",
	} {
		got := routeAnthropicModel(m)
		if got != routeOpenAIResponses {
			t.Errorf("model %q: expected routeOpenAIResponses, got %v", m, got)
		}
	}
}

func TestRouteAnthropicModel_UnknownReturnsUnknown(t *testing.T) {
	got := routeAnthropicModel("totally-made-up-model")
	if got != routeUnknown {
		t.Fatalf("expected routeUnknown, got %v", got)
	}
}

// --------------------------------------------------------------------------
// /v1/messages dispatches to OpenAI when the model is GPT-family
// --------------------------------------------------------------------------

// MessagesDispatcher is the type the server uses to route /v1/messages to
// either Bedrock (Claude models) or OpenAI Chat / Responses (GPT/o3/o4
// models). Tests construct it with both transports pointing at fake
// upstreams.
func newMessagesDispatcherForTest(bt *BedrockTransport, ot *OpenAITransport) *MessagesDispatcher {
	return &MessagesDispatcher{Bedrock: bt, OpenAI: ot}
}

func TestMessagesDispatcher_RoutesGPTToOpenAIChat(t *testing.T) {
	bedrockUp := newFakeBedrockUpstream()
	defer bedrockUp.close()
	openaiUp := newFakeOpenAIUpstream()
	defer openaiUp.close()

	bt := newBedrockTransportForTest(bedrockUp.server.URL, "BEDROCK")
	ot := newOpenAITransportForTest(openaiUp.server.URL, "OPENAI")
	d := newMessagesDispatcherForTest(bt, ot)

	body := `{"model":"gpt-5.5","max_tokens":8,"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	rec := httptest.NewRecorder()
	d.HandleAnthropicMessages(rec, req)

	bedrockPath, _, _, _ := bedrockUp.snapshot()
	if bedrockPath != "" {
		t.Fatalf("expected Bedrock NOT called, got path %q", bedrockPath)
	}
	openaiPath, _, headers, upBody := openaiUp.snapshot()
	if openaiPath != "/chat/completions" {
		t.Fatalf("expected OpenAI Chat Completions, got %q", openaiPath)
	}
	if got := headers.Get("Authorization"); got != "Bearer OPENAI" {
		t.Fatalf("expected Authorization=Bearer OPENAI, got %q", got)
	}
	var sent map[string]any
	if err := json.Unmarshal(upBody, &sent); err != nil {
		t.Fatalf("upstream body not JSON: %v", err)
	}
	if _, ok := sent["max_tokens"]; ok {
		t.Fatalf("expected max_tokens stripped after translation, got %v", sent)
	}
	if v, ok := sent["max_completion_tokens"]; !ok || int(v.(float64)) != 8 {
		t.Fatalf("expected max_completion_tokens=8, got %v", sent["max_completion_tokens"])
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from fake OpenAI upstream, got %d (%q)", rec.Code, rec.Body.String())
	}
}

func TestMessagesDispatcher_RoutesCodexToOpenAIResponses(t *testing.T) {
	bedrockUp := newFakeBedrockUpstream()
	defer bedrockUp.close()
	openaiUp := newFakeOpenAIUpstream()
	openaiUp.respBody = []byte(`{"id":"resp_test","object":"response","model":"gpt-5-codex","status":"completed","output":[]}`)
	defer openaiUp.close()

	bt := newBedrockTransportForTest(bedrockUp.server.URL, "BEDROCK")
	ot := newOpenAITransportForTest(openaiUp.server.URL, "OPENAI")
	d := newMessagesDispatcherForTest(bt, ot)

	// gpt-5-codex routes to /v1/responses; we still accept Anthropic Messages
	// shape on input.
	body := `{"model":"gpt-5-codex","max_tokens":8,"messages":[{"role":"user","content":"refactor this"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	rec := httptest.NewRecorder()
	d.HandleAnthropicMessages(rec, req)

	openaiPath, _, _, _ := openaiUp.snapshot()
	if openaiPath != "/responses" {
		t.Fatalf("expected OpenAI Responses dispatch for codex, got %q", openaiPath)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from fake Responses upstream, got %d (%q)", rec.Code, rec.Body.String())
	}
}

func TestMessagesDispatcher_RoutesClaudeToBedrock(t *testing.T) {
	bedrockUp := newFakeBedrockUpstream()
	defer bedrockUp.close()
	openaiUp := newFakeOpenAIUpstream()
	defer openaiUp.close()

	bt := newBedrockTransportForTest(bedrockUp.server.URL, "BEDROCK")
	ot := newOpenAITransportForTest(openaiUp.server.URL, "OPENAI")
	d := newMessagesDispatcherForTest(bt, ot)

	body := `{"model":"us.anthropic.claude-opus-4-7","max_tokens":8,"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	rec := httptest.NewRecorder()
	d.HandleAnthropicMessages(rec, req)

	bedrockPath, _, _, _ := bedrockUp.snapshot()
	if bedrockPath != "/model/us.anthropic.claude-opus-4-7/invoke" {
		t.Fatalf("expected Bedrock invoke, got %q", bedrockPath)
	}
	openaiPath, _, _, _ := openaiUp.snapshot()
	if openaiPath != "" {
		t.Fatalf("expected OpenAI NOT called, got %q", openaiPath)
	}
}

func TestMessagesDispatcher_RejectsUnknownModel(t *testing.T) {
	bedrockUp := newFakeBedrockUpstream()
	defer bedrockUp.close()
	openaiUp := newFakeOpenAIUpstream()
	defer openaiUp.close()

	bt := newBedrockTransportForTest(bedrockUp.server.URL, "BEDROCK")
	ot := newOpenAITransportForTest(openaiUp.server.URL, "OPENAI")
	d := newMessagesDispatcherForTest(bt, ot)

	body := `{"model":"totally-fake-model","max_tokens":8,"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	rec := httptest.NewRecorder()
	d.HandleAnthropicMessages(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on unknown model, got %d (%q)", rec.Code, rec.Body.String())
	}
}

func TestMessagesDispatcher_RejectsMissingModel(t *testing.T) {
	d := newMessagesDispatcherForTest(
		newBedrockTransportForTest("http://x.invalid", "X"),
		newOpenAITransportForTest("http://y.invalid", "Y"),
	)
	body := `{"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	rec := httptest.NewRecorder()
	d.HandleAnthropicMessages(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestMessagesDispatcher_RejectsNonPost(t *testing.T) {
	d := newMessagesDispatcherForTest(
		newBedrockTransportForTest("http://x.invalid", "X"),
		newOpenAITransportForTest("http://y.invalid", "Y"),
	)
	req := httptest.NewRequest(http.MethodGet, "/v1/messages", nil)
	rec := httptest.NewRecorder()
	d.HandleAnthropicMessages(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestMessagesDispatcher_RejectsNonJSON(t *testing.T) {
	d := newMessagesDispatcherForTest(
		newBedrockTransportForTest("http://x.invalid", "X"),
		newOpenAITransportForTest("http://y.invalid", "Y"),
	)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader("not json"))
	rec := httptest.NewRecorder()
	d.HandleAnthropicMessages(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestNewOpenAITransport_TrimsTrailingSlash(t *testing.T) {
	ot := NewOpenAITransport("https://example.invalid/", "BEAR", nil)
	if ot.UpstreamBaseURL != "https://example.invalid" {
		t.Fatalf("expected trailing slash trimmed, got %q", ot.UpstreamBaseURL)
	}
	if ot.HTTPClient == nil {
		t.Fatal("expected default HTTP client to be set")
	}
}

// --------------------------------------------------------------------------
// Image content block translation
// --------------------------------------------------------------------------

func TestTranslateAnthropicToOpenAIChat_ImageBase64Block(t *testing.T) {
	// Anthropic image content block (base64) → OpenAI image_url with data URL.
	in := []byte(`{
		"model":"gpt-4o",
		"max_tokens":8,
		"messages":[
			{
				"role":"user",
				"content":[
					{"type":"text","text":"What is in this image?"},
					{"type":"image","source":{"type":"base64","media_type":"image/png","data":"iVBORw0KGgo="}}
				]
			}
		]
	}`)
	out, err := translateAnthropicToOpenAIChat(in)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	var got map[string]any
	_ = json.Unmarshal(out, &got)
	msgs := got["messages"].([]any)
	user := msgs[0].(map[string]any)
	parts, ok := user["content"].([]any)
	if !ok || len(parts) != 2 {
		t.Fatalf("expected content parts array, got %v", user["content"])
	}
	imgPart := parts[1].(map[string]any)
	if imgPart["type"] != "image_url" {
		t.Fatalf("expected type=image_url, got %v", imgPart)
	}
	imgURL := imgPart["image_url"].(map[string]any)
	if !strings.HasPrefix(imgURL["url"].(string), "data:image/png;base64,") {
		t.Fatalf("expected data URL prefix, got %v", imgURL["url"])
	}
}

func TestTranslateAnthropicToOpenAIChat_ImageURLBlock(t *testing.T) {
	in := []byte(`{
		"model":"gpt-4o",
		"max_tokens":8,
		"messages":[
			{
				"role":"user",
				"content":[
					{"type":"image","source":{"type":"url","url":"https://example.invalid/img.png"}}
				]
			}
		]
	}`)
	out, err := translateAnthropicToOpenAIChat(in)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	var got map[string]any
	_ = json.Unmarshal(out, &got)
	msgs := got["messages"].([]any)
	user := msgs[0].(map[string]any)
	parts, ok := user["content"].([]any)
	if !ok || len(parts) != 1 {
		t.Fatalf("expected single image part, got %v", user["content"])
	}
	imgURL := parts[0].(map[string]any)["image_url"].(map[string]any)
	if imgURL["url"] != "https://example.invalid/img.png" {
		t.Fatalf("expected URL preserved, got %v", imgURL["url"])
	}
}

// --------------------------------------------------------------------------
// Tool-result block: multiple text sub-blocks concatenated
// --------------------------------------------------------------------------

func TestTranslateAnthropicToOpenAIChat_ToolResultMultiTextBlocks(t *testing.T) {
	in := []byte(`{
		"model":"gpt-5.5",
		"max_tokens":8,
		"messages":[
			{
				"role":"user",
				"content":[
					{
						"type":"tool_result",
						"tool_use_id":"tu_2",
						"content":[
							{"type":"text","text":"line 1"},
							{"type":"text","text":"line 2"}
						]
					}
				]
			}
		]
	}`)
	out, err := translateAnthropicToOpenAIChat(in)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	var got map[string]any
	_ = json.Unmarshal(out, &got)
	msgs := got["messages"].([]any)
	tm := msgs[0].(map[string]any)
	if tm["role"] != "tool" || tm["tool_call_id"] != "tu_2" {
		t.Fatalf("expected tool message with tu_2, got %v", tm)
	}
	if !strings.Contains(tm["content"].(string), "line 1") || !strings.Contains(tm["content"].(string), "line 2") {
		t.Fatalf("expected concatenated text content, got %v", tm["content"])
	}
}

// --------------------------------------------------------------------------
// Anthropic Messages → OpenAI Responses translator
// --------------------------------------------------------------------------

func TestTranslateAnthropicToOpenAIResponses_BasicShape(t *testing.T) {
	in := []byte(`{
		"model":"gpt-5-codex",
		"max_tokens":256,
		"system":"You are an expert refactor assistant.",
		"messages":[{"role":"user","content":"refactor"}],
		"stop_sequences":["END"],
		"anthropic_version":"bedrock-2023-05-31"
	}`)
	out, err := translateAnthropicToOpenAIResponses(in)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	var got map[string]any
	_ = json.Unmarshal(out, &got)
	if _, ok := got["anthropic_version"]; ok {
		t.Fatalf("expected anthropic_version stripped")
	}
	if got["model"] != "gpt-5-codex" {
		t.Fatalf("expected model preserved, got %v", got["model"])
	}
	if got["instructions"] != "You are an expert refactor assistant." {
		t.Fatalf("expected instructions=system text, got %v", got["instructions"])
	}
	if _, ok := got["system"]; ok {
		t.Fatalf("expected system removed")
	}
	if v, ok := got["max_output_tokens"].(float64); !ok || int(v) != 256 {
		t.Fatalf("expected max_output_tokens=256, got %v", got["max_output_tokens"])
	}
	if _, ok := got["max_tokens"]; ok {
		t.Fatalf("expected max_tokens removed")
	}
	if _, ok := got["messages"]; ok {
		t.Fatalf("expected messages renamed to input")
	}
	input := got["input"].([]any)
	if len(input) != 1 {
		t.Fatalf("expected 1 input message, got %d", len(input))
	}
	if got["stop"] == nil {
		t.Fatalf("expected stop populated, got %v", got)
	}
}

func TestTranslateAnthropicToOpenAIResponses_ToolsTranslated(t *testing.T) {
	in := []byte(`{
		"model":"gpt-5-codex",
		"max_tokens":8,
		"messages":[{"role":"user","content":"hi"}],
		"tools":[{"name":"run_shell","description":"Run a shell cmd.","input_schema":{"type":"object","properties":{"cmd":{"type":"string"}}}}]
	}`)
	out, err := translateAnthropicToOpenAIResponses(in)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	var got map[string]any
	_ = json.Unmarshal(out, &got)
	tools := got["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %v", got["tools"])
	}
	t0 := tools[0].(map[string]any)
	fn := t0["function"].(map[string]any)
	if fn["name"] != "run_shell" {
		t.Fatalf("expected function.name=run_shell, got %v", fn)
	}
}

func TestTranslateAnthropicToOpenAIChat_RejectsNonJSON(t *testing.T) {
	_, err := translateAnthropicToOpenAIChat([]byte("not json"))
	if err == nil {
		t.Fatal("expected error on non-JSON input")
	}
}

func TestTranslateAnthropicToOpenAIResponses_RejectsNonJSON(t *testing.T) {
	_, err := translateAnthropicToOpenAIResponses([]byte("not json"))
	if err == nil {
		t.Fatal("expected error on non-JSON input")
	}
}

// --------------------------------------------------------------------------
// Edge cases on routeAnthropicModel
// --------------------------------------------------------------------------

func TestRouteAnthropicModel_EmptyAndWhitespace(t *testing.T) {
	for _, m := range []string{"", "   ", "\t"} {
		if got := routeAnthropicModel(m); got != routeUnknown {
			t.Errorf("model %q: expected routeUnknown, got %v", m, got)
		}
	}
}
