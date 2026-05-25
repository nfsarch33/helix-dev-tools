package zdproxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// bedrockAnthropicVersion is the value AWS Bedrock requires in the
// `anthropic_version` body field. Claude Code CLI / SDK already populate it,
// but we inject the known-good default when the field is absent so the proxy
// is robust against minimal Anthropic-Messages payloads.
const bedrockAnthropicVersion = "bedrock-2023-05-31"

// BedrockTransport routes Anthropic-Messages-style requests to the ZD Bedrock
// surface of the AI gateway. It supports two inbound shapes:
//
//   - Anthropic Messages: POST /v1/messages with `model` in body. The model
//     id is extracted from the JSON body and the request is forwarded to
//     ${UpstreamBaseURL}/model/{id}/invoke (or invoke-with-response-stream
//     when the body contains "stream": true).
//   - Bedrock-shape passthrough: POST /bedrock/model/{id}/invoke or
//     /bedrock/model/{id}/invoke-with-response-stream. Bodies are forwarded
//     byte-for-byte; the model id and endpoint come from the URL path.
//
// All forwards add Authorization: Bearer <UpstreamBearer>. Responses stream
// back to the caller via incremental Read+Write+Flush so AWS event-stream
// binary frames pass through unchanged. The transport is a security and
// observability shim, not a transcoder.
//
// The transport never echoes UpstreamBearer in any response or log. Upstream
// connection errors return a generic 502; non-2xx upstream responses are
// proxied through with their body (the upstream gateway's error envelopes
// never contain our bearer).
type BedrockTransport struct {
	UpstreamBaseURL string
	UpstreamBearer  string
	HTTPClient      *http.Client

	// Metrics, when non-nil, is updated on each forwarded request.
	Metrics *Metrics
}

// NewBedrockTransport returns a BedrockTransport with sane defaults. The
// UpstreamBaseURL has any trailing slash trimmed so URL composition is
// deterministic.
func NewBedrockTransport(upstreamBaseURL, bearer string, client *http.Client) *BedrockTransport {
	if client == nil {
		client = &http.Client{} // no global timeout: streaming requests can be long
	}
	return &BedrockTransport{
		UpstreamBaseURL: strings.TrimRight(upstreamBaseURL, "/"),
		UpstreamBearer:  bearer,
		HTTPClient:      client,
	}
}

// HandleAnthropicMessages serves POST /v1/messages by parsing the JSON body
// to extract the model id and stream flag, injecting the
// `anthropic_version` field if absent, then forwarding to the matching
// Bedrock invoke endpoint.
func (b *BedrockTransport) HandleAnthropicMessages(w http.ResponseWriter, r *http.Request) {
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
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_json", "request body must be JSON")
		return
	}
	if probe.Model == "" {
		writeJSONError(w, http.StatusBadRequest, "missing_model", "request body must include `model`")
		return
	}

	body, err = prepareBedrockBody(body)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "prepare_body", "failed to prepare Bedrock body")
		return
	}

	endpoint := "invoke"
	route := "bedrock_invoke"
	if probe.Stream {
		endpoint = "invoke-with-response-stream"
		route = "bedrock_invoke_stream"
	}
	upstreamURL := fmt.Sprintf("%s/model/%s/%s", b.UpstreamBaseURL, probe.Model, endpoint)
	b.forward(w, r, upstreamURL, bytes.NewReader(body), route, probe.Model)
}

// HandleBedrockPassthrough serves POST /bedrock/model/{id}/invoke or
// /bedrock/model/{id}/invoke-with-response-stream. The model id and endpoint
// come from the URL path. The request body is forwarded byte-for-byte (it is
// already a Bedrock-shape payload).
func (b *BedrockTransport) HandleBedrockPassthrough(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required")
		return
	}
	modelID, endpoint, ok := parseBedrockPassthroughPath(r.URL.Path)
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_path", "expected /bedrock/model/{id}/invoke[-with-response-stream]")
		return
	}

	raw, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "read_body", "failed to read request body")
		return
	}
	prepared, err := prepareBedrockBody(raw)
	if err != nil {
		prepared = raw
	}

	upstreamURL := fmt.Sprintf("%s/model/%s/%s", b.UpstreamBaseURL, modelID, endpoint)
	route := "bedrock_passthrough"
	if endpoint == "invoke-with-response-stream" {
		route = "bedrock_passthrough_stream"
	}
	b.forward(w, r, upstreamURL, bytes.NewReader(prepared), route, modelID)
}

// forward executes the upstream request and streams the response body back
// to the client. The response Content-Type is preserved so AWS event-stream
// frames pass through unchanged. Upstream errors (connection failure) are
// surfaced as 502 with a generic message; the upstream bearer is never
// included in any response body.
//
// route and model label the metrics emission so a single Grafana panel can
// split timings between non-streaming /invoke and streaming
// /invoke-with-response-stream and between Claude Opus and Haiku families.
func (b *BedrockTransport) forward(w http.ResponseWriter, r *http.Request, upstreamURL string, body io.Reader, route, model string) {
	if b.Metrics != nil {
		b.Metrics.BeginInflight(route)
		defer b.Metrics.EndInflight(route)
	}
	start := time.Now()

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, upstreamURL, body)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "build_request", "failed to build upstream request")
		if b.Metrics != nil {
			b.Metrics.RecordRequest(route, model, http.StatusInternalServerError, time.Since(start))
		}
		return
	}
	req.Header.Set("Authorization", "Bearer "+b.UpstreamBearer)
	req.Header.Set("Content-Type", "application/json")
	if accept := r.Header.Get("Accept"); accept != "" {
		req.Header.Set("Accept", accept)
	}

	resp, err := b.HTTPClient.Do(req)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, "upstream_error", "upstream request failed")
		if b.Metrics != nil {
			b.Metrics.RecordRequest(route, model, http.StatusBadGateway, time.Since(start))
		}
		return
	}
	defer resp.Body.Close()

	for _, h := range []string{
		"Content-Type",
		"X-Amzn-Bedrock-Input-Token-Count",
		"X-Amzn-Bedrock-Output-Token-Count",
		"X-Amzn-Requestid",
	} {
		if v := resp.Header.Get(h); v != "" {
			w.Header().Set(h, v)
		}
	}
	if b.Metrics != nil {
		if n, err := strconv.Atoi(resp.Header.Get("X-Amzn-Bedrock-Input-Token-Count")); err == nil {
			b.Metrics.RecordTokens(model, "input", n)
		}
		if n, err := strconv.Atoi(resp.Header.Get("X-Amzn-Bedrock-Output-Token-Count")); err == nil {
			b.Metrics.RecordTokens(model, "output", n)
		}
	}
	w.WriteHeader(resp.StatusCode)

	streamCopy(w, resp.Body)
	if b.Metrics != nil {
		b.Metrics.RecordRequest(route, model, resp.StatusCode, time.Since(start))
	}
}

// streamCopy forwards bytes from src to dst, flushing dst after every chunk
// when dst implements http.Flusher. This keeps AWS event-stream binary frames
// flowing to the Anthropic SDK without buffering the whole response.
func streamCopy(dst http.ResponseWriter, src io.Reader) {
	flusher, _ := dst.(http.Flusher)
	buf := make([]byte, 32*1024)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if err != nil {
			return
		}
	}
}

// prepareBedrockBody normalizes an Anthropic-Messages-shaped body for the
// Bedrock invoke endpoint:
//   - injects anthropic_version when absent (Bedrock requires it)
//   - removes the top-level `model` key (Bedrock keys model from the URL
//     path; sending it in the body returns 400 "extraneous key [model] is
//     not permitted")
//   - removes the top-level `stream` key (passing stream=true to the
//     non-streaming /invoke endpoint causes 400 in some gateway versions;
//     the streaming endpoint is reached via URL routing in the caller)
func prepareBedrockBody(body []byte) ([]byte, error) {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	delete(m, "model")
	delete(m, "stream")
	if _, ok := m["anthropic_version"]; !ok {
		m["anthropic_version"] = bedrockAnthropicVersion
	}
	return json.Marshal(m)
}

// parseBedrockPassthroughPath returns the model id and endpoint segment for a
// request path of the form /bedrock/model/{id}/invoke[-with-response-stream].
// The endpoint must be exactly "invoke" or "invoke-with-response-stream".
func parseBedrockPassthroughPath(p string) (modelID, endpoint string, ok bool) {
	rest := strings.TrimPrefix(p, "/bedrock/")
	if rest == p {
		return "", "", false
	}
	parts := strings.Split(rest, "/")
	if len(parts) != 3 || parts[0] != "model" || parts[1] == "" {
		return "", "", false
	}
	switch parts[2] {
	case "invoke", "invoke-with-response-stream":
		return parts[1], parts[2], true
	}
	return "", "", false
}

// writeJSONError emits a small, predictable error envelope. It never includes
// dynamic interpolation of upstream credentials.
func writeJSONError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	body := map[string]any{
		"error": map[string]string{
			"type":    code,
			"message": message,
		},
	}
	_ = json.NewEncoder(w).Encode(body)
}
