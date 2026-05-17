package compression_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/mcpfilter/compression"
)

func TestE2E_RoundTripPreservesCriticalFields(t *testing.T) {
	t.Parallel()
	c := compression.New(compression.Config{MaxArrayLen: 5, MaxDepth: 6, MaxStringLen: 500})

	input := map[string]any{
		"id":      "tool-result-123",
		"type":    "tool_result",
		"status":  "success",
		"content": strings.Repeat("x", 1000),
		"metadata": map[string]any{
			"model":  "claude-4-opus",
			"tokens": 42,
		},
		"results": make([]any, 30),
	}
	for i := range input["results"].([]any) {
		input["results"].([]any)[i] = map[string]any{"row": i, "val": "data"}
	}

	raw, _ := json.Marshal(input)
	compressed := c.Compress(raw)

	ratio := compression.Ratio(raw, compressed)
	if ratio < 0.3 {
		t.Errorf("ratio=%.2f, want >=0.3 (at least 30%% reduction)", ratio)
	}

	var out map[string]any
	if err := json.Unmarshal(compressed, &out); err != nil {
		t.Fatalf("compressed output is not valid JSON: %v", err)
	}

	if out["id"] != "tool-result-123" {
		t.Errorf("id=%v, want tool-result-123", out["id"])
	}
	if out["type"] != "tool_result" {
		t.Errorf("type=%v, want tool_result", out["type"])
	}
	if out["status"] != "success" {
		t.Errorf("status=%v, want success", out["status"])
	}

	meta, ok := out["metadata"].(map[string]any)
	if !ok {
		t.Fatal("metadata lost during compression")
	}
	if meta["model"] != "claude-4-opus" {
		t.Errorf("metadata.model=%v, want claude-4-opus", meta["model"])
	}

	content, ok := out["content"].(string)
	if !ok {
		t.Fatal("content field lost")
	}
	if !strings.Contains(content, "truncated") {
		t.Error("long content should be truncated with notice")
	}
	if !strings.Contains(content, "1000 total chars") {
		t.Error("truncation notice should include original length")
	}

	results, ok := out["results"].(map[string]any)
	if !ok {
		t.Fatal("results should be wrapped in truncation envelope")
	}
	if results["_truncated"] != true {
		t.Error("results should be marked truncated")
	}
	if results["_total"] != float64(30) {
		t.Errorf("_total=%v, want 30", results["_total"])
	}
}

func TestE2E_CompressDecompressDepthSafe(t *testing.T) {
	t.Parallel()
	c := compression.New(compression.Config{MaxArrayLen: 10, MaxDepth: 4, MaxStringLen: 2000})

	deep := map[string]any{
		"l1": map[string]any{
			"l2": map[string]any{
				"l3": map[string]any{
					"l4": map[string]any{
						"l5": map[string]any{
							"l6": "should be replaced by depth hint",
						},
					},
				},
			},
		},
	}

	raw, _ := json.Marshal(deep)
	compressed := c.Compress(raw)

	var out map[string]any
	if err := json.Unmarshal(compressed, &out); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}

	l1 := out["l1"].(map[string]any)
	l2 := l1["l2"].(map[string]any)
	l3 := l2["l3"].(map[string]any)
	l4 := l3["l4"].(map[string]any)
	hint, ok := l4["l5"].(string)
	if !ok {
		t.Fatalf("l5 should be replaced by depth hint string, got %T", l4["l5"])
	}
	if !strings.Contains(hint, "depth limit") {
		t.Errorf("depth hint = %q, want 'depth limit' message", hint)
	}
}

func TestE2E_LargeMCPResponseCompression(t *testing.T) {
	t.Parallel()
	c := compression.New(compression.DefaultConfig())

	resp := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"result": map[string]any{
			"content": []any{
				map[string]any{
					"type": "text",
					"text": strings.Repeat("MCP tool output line\n", 200),
				},
			},
		},
	}

	raw, _ := json.Marshal(resp)
	compressed := c.Compress(raw)

	var out map[string]any
	if err := json.Unmarshal(compressed, &out); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}

	if out["jsonrpc"] != "2.0" {
		t.Error("jsonrpc field must be preserved")
	}
	result, ok := out["result"].(map[string]any)
	if !ok {
		t.Fatal("result field must be preserved")
	}
	content, ok := result["content"].([]any)
	if !ok {
		t.Fatal("result.content must be preserved as array")
	}
	if len(content) != 1 {
		t.Errorf("content length=%d, want 1", len(content))
	}

	ratio := compression.Ratio(raw, compressed)
	if ratio < 0.5 {
		t.Errorf("large MCP response compression ratio=%.2f, want >=0.5", ratio)
	}
}
