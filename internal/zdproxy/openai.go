package zdproxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// OpenAITransport routes OpenAI Chat Completions and Responses requests to
// the ZD OpenAI gateway. It supports two complementary use cases:
//
//   - Cursor (and other OpenAI-shape callers) on the MacBook send native
//     OpenAI-shape requests; HandleChatCompletions / HandleResponses are
//     transparent passthroughs (body forwarded byte-for-byte).
//   - Claude Code CLI / Desktop Code-runtime children speak Anthropic
//     Messages and select an OpenAI model id (e.g. "gpt-5.5" or
//     "gpt-5-codex"). The MessagesDispatcher routes those through the
//     translator entrypoints below.
//
// All forwards add Authorization: Bearer <UpstreamBearer>. Responses stream
// back via incremental Read+Write+Flush so OpenAI SSE chunks pass through
// unchanged. The upstream bearer is never echoed in any response or log.
type OpenAITransport struct {
	UpstreamBaseURL string
	UpstreamBearer  string
	HTTPClient      *http.Client
}

// NewOpenAITransport returns an OpenAITransport with a sane default client.
// The trailing slash on UpstreamBaseURL is trimmed so URL composition is
// deterministic.
func NewOpenAITransport(upstreamBaseURL, bearer string, client *http.Client) *OpenAITransport {
	if client == nil {
		client = &http.Client{} // no global timeout: streaming requests can be long
	}
	return &OpenAITransport{
		UpstreamBaseURL: strings.TrimRight(upstreamBaseURL, "/"),
		UpstreamBearer:  bearer,
		HTTPClient:      client,
	}
}

// HandleChatCompletions serves POST /v1/chat/completions as a transparent
// passthrough to ${UpstreamBaseURL}/v1/chat/completions.
func (o *OpenAITransport) HandleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required")
		return
	}
	o.forward(w, r, o.UpstreamBaseURL+"/v1/chat/completions", r.Body)
}

// HandleResponses serves POST /v1/responses as a transparent passthrough to
// ${UpstreamBaseURL}/v1/responses (used for o3/o4/codex/pro models).
func (o *OpenAITransport) HandleResponses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required")
		return
	}
	o.forward(w, r, o.UpstreamBaseURL+"/v1/responses", r.Body)
}

// ForwardChatTranslated translates an Anthropic-Messages-shape body to OpenAI
// Chat Completions shape and forwards it. Used by the MessagesDispatcher
// when a /v1/messages request selects a GPT-family model.
func (o *OpenAITransport) ForwardChatTranslated(w http.ResponseWriter, r *http.Request, anthropicBody []byte) {
	translated, err := translateAnthropicToOpenAIChat(anthropicBody)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "translate_chat", "failed to translate Anthropic body to OpenAI Chat Completions")
		return
	}
	o.forward(w, r, o.UpstreamBaseURL+"/v1/chat/completions", bytes.NewReader(translated))
}

// ForwardResponsesTranslated translates an Anthropic-Messages-shape body to
// OpenAI Responses shape and forwards it. Used for codex/o3/o4/pro models.
func (o *OpenAITransport) ForwardResponsesTranslated(w http.ResponseWriter, r *http.Request, anthropicBody []byte) {
	translated, err := translateAnthropicToOpenAIResponses(anthropicBody)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "translate_responses", "failed to translate Anthropic body to OpenAI Responses")
		return
	}
	o.forward(w, r, o.UpstreamBaseURL+"/v1/responses", bytes.NewReader(translated))
}

// forward executes the upstream request and streams the response body back
// to the client, mirroring BedrockTransport.forward. The upstream bearer is
// never included in any response body.
func (o *OpenAITransport) forward(w http.ResponseWriter, r *http.Request, upstreamURL string, body io.Reader) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, upstreamURL, body)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "build_request", "failed to build upstream request")
		return
	}
	req.Header.Set("Authorization", "Bearer "+o.UpstreamBearer)
	req.Header.Set("Content-Type", "application/json")
	if accept := r.Header.Get("Accept"); accept != "" {
		req.Header.Set("Accept", accept)
	}

	resp, err := o.HTTPClient.Do(req)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, "upstream_error", "upstream request failed")
		return
	}
	defer resp.Body.Close()

	for _, h := range []string{
		"Content-Type",
		"X-Request-ID",
		"X-Ratelimit-Remaining-Tokens",
		"X-Ratelimit-Remaining-Requests",
	} {
		if v := resp.Header.Get(h); v != "" {
			w.Header().Set(h, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	streamCopy(w, resp.Body)
}

// --------------------------------------------------------------------------
// Pure translator: Anthropic Messages → OpenAI Chat Completions
// --------------------------------------------------------------------------

// translateAnthropicToOpenAIChat returns an OpenAI Chat Completions body
// derived from an Anthropic Messages body. The translation:
//
//   - removes top-level `anthropic_version` (OpenAI rejects unknown keys)
//   - renames `max_tokens` → `max_completion_tokens` (gpt-5 family
//     enforces this)
//   - renames `stop_sequences` → `stop`
//   - flattens top-level `system` (string or content-block array) into a
//     leading message with role=system
//   - rewrites `tools[].input_schema` → `tools[].function.parameters`
//   - rewrites assistant content blocks containing `tool_use` → assistant
//     message with `tool_calls`
//   - rewrites user content blocks containing `tool_result` → message with
//     role=tool and `tool_call_id`
//
// On JSON parse failure the function returns an error so callers can
// surface a 400 to the client. The function is pure: no I/O, no globals.
func translateAnthropicToOpenAIChat(body []byte) ([]byte, error) {
	var src map[string]any
	if err := json.Unmarshal(body, &src); err != nil {
		return nil, fmt.Errorf("invalid JSON body: %w", err)
	}

	delete(src, "anthropic_version")

	if mt, ok := src["max_tokens"]; ok {
		src["max_completion_tokens"] = mt
		delete(src, "max_tokens")
	}
	if ss, ok := src["stop_sequences"]; ok {
		src["stop"] = ss
		delete(src, "stop_sequences")
	}

	systemMessage := buildSystemMessage(src["system"])
	delete(src, "system")

	if rawMsgs, ok := src["messages"].([]any); ok {
		converted := make([]any, 0, len(rawMsgs)+1)
		if systemMessage != nil {
			converted = append(converted, systemMessage)
		}
		for _, m := range rawMsgs {
			obj, ok := m.(map[string]any)
			if !ok {
				converted = append(converted, m)
				continue
			}
			converted = append(converted, translateAnthropicMessage(obj)...)
		}
		src["messages"] = converted
	} else if systemMessage != nil {
		src["messages"] = []any{systemMessage}
	}

	if rawTools, ok := src["tools"].([]any); ok {
		translated := make([]any, 0, len(rawTools))
		for _, t := range rawTools {
			tObj, ok := t.(map[string]any)
			if !ok {
				continue
			}
			translated = append(translated, translateAnthropicTool(tObj))
		}
		src["tools"] = translated
	}

	return json.Marshal(src)
}

// buildSystemMessage returns the OpenAI system-role message for the given
// Anthropic top-level `system` field, or nil if absent. The Anthropic field
// can be a plain string or an array of `{type: text, text: ...}` blocks.
func buildSystemMessage(raw any) map[string]any {
	switch v := raw.(type) {
	case nil:
		return nil
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		return map[string]any{"role": "system", "content": v}
	case []any:
		var sb strings.Builder
		for i, item := range v {
			block, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if block["type"] != "text" {
				continue
			}
			text, _ := block["text"].(string)
			if text == "" {
				continue
			}
			if i > 0 && sb.Len() > 0 {
				sb.WriteString("\n\n")
			}
			sb.WriteString(text)
		}
		if sb.Len() == 0 {
			return nil
		}
		return map[string]any{"role": "system", "content": sb.String()}
	default:
		return nil
	}
}

// translateAnthropicMessage rewrites one Anthropic message into one or more
// OpenAI messages. A user message containing only tool_result blocks
// becomes one or more `role: tool` messages. An assistant message
// containing a mix of text and tool_use blocks becomes a single assistant
// message with `content` (concatenated text) and `tool_calls`.
func translateAnthropicMessage(msg map[string]any) []any {
	role, _ := msg["role"].(string)
	content := msg["content"]

	switch c := content.(type) {
	case string:
		return []any{map[string]any{"role": role, "content": c}}
	case []any:
		// Inspect blocks; partition into text / tool_use / tool_result.
		var (
			textParts []string
			toolCalls []any
			toolMsgs  []any
			imageURLs []map[string]any
		)
		for _, raw := range c {
			block, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			switch block["type"] {
			case "text":
				if t, _ := block["text"].(string); t != "" {
					textParts = append(textParts, t)
				}
			case "tool_use":
				if tc := translateToolUseBlock(block); tc != nil {
					toolCalls = append(toolCalls, tc)
				}
			case "tool_result":
				if tm := translateToolResultBlock(block); tm != nil {
					toolMsgs = append(toolMsgs, tm)
				}
			case "image":
				if iu := translateImageBlock(block); iu != nil {
					imageURLs = append(imageURLs, iu)
				}
			}
		}

		// User-side tool_result blocks become role=tool messages, replacing
		// the user message itself.
		if role == "user" && len(toolMsgs) > 0 && len(textParts) == 0 && len(imageURLs) == 0 {
			return toolMsgs
		}

		out := map[string]any{"role": role}
		if len(imageURLs) > 0 {
			parts := make([]any, 0, len(textParts)+len(imageURLs))
			for _, t := range textParts {
				parts = append(parts, map[string]any{"type": "text", "text": t})
			}
			for _, iu := range imageURLs {
				parts = append(parts, iu)
			}
			out["content"] = parts
		} else {
			out["content"] = strings.Join(textParts, "\n\n")
		}
		if len(toolCalls) > 0 {
			out["tool_calls"] = toolCalls
		}

		// User message with only tool_result handled above; otherwise emit
		// the synthesised message followed by any trailing tool messages.
		result := []any{out}
		result = append(result, toolMsgs...)
		return result
	default:
		return []any{msg}
	}
}

func translateToolUseBlock(block map[string]any) map[string]any {
	id, _ := block["id"].(string)
	name, _ := block["name"].(string)
	input := block["input"]
	args, err := json.Marshal(input)
	if err != nil {
		args = []byte(`{}`)
	}
	return map[string]any{
		"id":   id,
		"type": "function",
		"function": map[string]any{
			"name":      name,
			"arguments": string(args),
		},
	}
}

func translateToolResultBlock(block map[string]any) map[string]any {
	tcID, _ := block["tool_use_id"].(string)
	content := block["content"]
	var contentStr string
	switch v := content.(type) {
	case string:
		contentStr = v
	case []any:
		var sb strings.Builder
		for _, raw := range v {
			obj, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if obj["type"] == "text" {
				if t, _ := obj["text"].(string); t != "" {
					if sb.Len() > 0 {
						sb.WriteString("\n")
					}
					sb.WriteString(t)
				}
			}
		}
		contentStr = sb.String()
	default:
		if v != nil {
			b, _ := json.Marshal(v)
			contentStr = string(b)
		}
	}
	return map[string]any{
		"role":         "tool",
		"tool_call_id": tcID,
		"content":      contentStr,
	}
}

func translateImageBlock(block map[string]any) map[string]any {
	src, ok := block["source"].(map[string]any)
	if !ok {
		return nil
	}
	switch src["type"] {
	case "base64":
		mediaType, _ := src["media_type"].(string)
		data, _ := src["data"].(string)
		if data == "" {
			return nil
		}
		return map[string]any{
			"type": "image_url",
			"image_url": map[string]any{
				"url": fmt.Sprintf("data:%s;base64,%s", mediaType, data),
			},
		}
	case "url":
		url, _ := src["url"].(string)
		if url == "" {
			return nil
		}
		return map[string]any{
			"type":      "image_url",
			"image_url": map[string]any{"url": url},
		}
	}
	return nil
}

func translateAnthropicTool(tool map[string]any) map[string]any {
	name, _ := tool["name"].(string)
	desc, _ := tool["description"].(string)
	schema := tool["input_schema"]
	fn := map[string]any{"name": name}
	if desc != "" {
		fn["description"] = desc
	}
	if schema != nil {
		fn["parameters"] = schema
	}
	return map[string]any{
		"type":     "function",
		"function": fn,
	}
}

// --------------------------------------------------------------------------
// Pure translator: Anthropic Messages → OpenAI Responses (minimal)
// --------------------------------------------------------------------------

// translateAnthropicToOpenAIResponses returns an OpenAI Responses body
// derived from an Anthropic Messages body. The Responses API accepts an
// `input` field shaped like Chat messages and an `instructions` field for
// the system prompt; we map accordingly:
//
//   - top-level `system` → `instructions`
//   - `messages` → `input` (after the same per-message translation as Chat
//     Completions)
//   - `max_tokens` → `max_output_tokens`
//   - `stop_sequences` → `stop`
//   - removes `anthropic_version`
//
// Tool translation reuses the Chat translator since the Responses API
// accepts the same `tools[].function` shape for function tools.
func translateAnthropicToOpenAIResponses(body []byte) ([]byte, error) {
	var src map[string]any
	if err := json.Unmarshal(body, &src); err != nil {
		return nil, fmt.Errorf("invalid JSON body: %w", err)
	}

	delete(src, "anthropic_version")

	if mt, ok := src["max_tokens"]; ok {
		src["max_output_tokens"] = mt
		delete(src, "max_tokens")
	}
	if ss, ok := src["stop_sequences"]; ok {
		src["stop"] = ss
		delete(src, "stop_sequences")
	}

	if sys := src["system"]; sys != nil {
		if msg := buildSystemMessage(sys); msg != nil {
			if c, ok := msg["content"].(string); ok {
				src["instructions"] = c
			}
		}
		delete(src, "system")
	}

	if rawMsgs, ok := src["messages"].([]any); ok {
		converted := make([]any, 0, len(rawMsgs))
		for _, m := range rawMsgs {
			obj, ok := m.(map[string]any)
			if !ok {
				converted = append(converted, m)
				continue
			}
			converted = append(converted, translateAnthropicMessage(obj)...)
		}
		src["input"] = converted
		delete(src, "messages")
	}

	if rawTools, ok := src["tools"].([]any); ok {
		translated := make([]any, 0, len(rawTools))
		for _, t := range rawTools {
			tObj, ok := t.(map[string]any)
			if !ok {
				continue
			}
			translated = append(translated, translateAnthropicTool(tObj))
		}
		src["tools"] = translated
	}

	return json.Marshal(src)
}

// --------------------------------------------------------------------------
// Model dispatch (Bedrock vs OpenAI Chat vs OpenAI Responses)
// --------------------------------------------------------------------------

// anthropicRoute names the upstream surface a /v1/messages request should
// land on, based on the model id in the body.
type anthropicRoute int

const (
	routeUnknown anthropicRoute = iota
	routeBedrockInvoke
	routeOpenAIChat
	routeOpenAIResponses
)

// routeAnthropicModel inspects the model id and returns the matching
// upstream route. Unknown ids return routeUnknown so the caller can reject
// with 400 rather than guessing.
//
// The mapping is intentionally explicit so we can grow it as new ZD-gateway
// allowlist entries appear (e.g. when a new GPT-5.x mini lands). It must
// stay in sync with the gateway allowlist documented in
// `reports/research/zd-gateway-models-2026-04-28.md`.
func routeAnthropicModel(model string) anthropicRoute {
	m := strings.ToLower(strings.TrimSpace(model))
	if m == "" {
		return routeUnknown
	}

	// Bedrock-Claude families.
	if strings.HasPrefix(m, "claude-") ||
		strings.HasPrefix(m, "us.anthropic.") ||
		strings.HasPrefix(m, "eu.anthropic.") ||
		strings.HasPrefix(m, "anthropic.") {
		return routeBedrockInvoke
	}

	// OpenAI Responses-only families: codex, pro, deep-research, o3, o4.
	if strings.Contains(m, "codex") ||
		strings.Contains(m, "deep-research") ||
		strings.Contains(m, "-pro") ||
		strings.HasPrefix(m, "o3") ||
		strings.HasPrefix(m, "o4") {
		return routeOpenAIResponses
	}

	// OpenAI Chat Completions families: gpt-* (excluding the responses-only
	// variants matched above).
	if strings.HasPrefix(m, "gpt-") {
		return routeOpenAIChat
	}

	return routeUnknown
}

// --------------------------------------------------------------------------
// MessagesDispatcher: /v1/messages → Bedrock or OpenAI based on model id
// --------------------------------------------------------------------------

// MessagesDispatcher is the single entrypoint for POST /v1/messages. It
// reads the body once, inspects the `model` field, and delegates to either
// the Bedrock transport or the OpenAI transport. Callers must instantiate
// with both transports populated.
type MessagesDispatcher struct {
	Bedrock *BedrockTransport
	OpenAI  *OpenAITransport
}

// HandleAnthropicMessages serves POST /v1/messages with model-aware
// dispatch.
func (d *MessagesDispatcher) HandleAnthropicMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required")
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "read_body", "failed to read request body")
		return
	}
	var probe struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_json", "request body must be JSON")
		return
	}
	if probe.Model == "" {
		writeJSONError(w, http.StatusBadRequest, "missing_model", "request body must include `model`")
		return
	}

	switch routeAnthropicModel(probe.Model) {
	case routeBedrockInvoke:
		if d.Bedrock == nil {
			writeJSONError(w, http.StatusInternalServerError, "no_bedrock", "Bedrock transport not configured")
			return
		}
		// Re-attach the body and delegate.
		r2 := r.Clone(r.Context())
		r2.Body = io.NopCloser(bytes.NewReader(body))
		d.Bedrock.HandleAnthropicMessages(w, r2)

	case routeOpenAIChat:
		if d.OpenAI == nil {
			writeJSONError(w, http.StatusInternalServerError, "no_openai", "OpenAI transport not configured")
			return
		}
		d.OpenAI.ForwardChatTranslated(w, r, body)

	case routeOpenAIResponses:
		if d.OpenAI == nil {
			writeJSONError(w, http.StatusInternalServerError, "no_openai", "OpenAI transport not configured")
			return
		}
		d.OpenAI.ForwardResponsesTranslated(w, r, body)

	default:
		writeJSONError(w, http.StatusBadRequest, "unknown_model", "model not in zd-gateway allowlist")
	}
}
