package compression_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/mcpfilter/compression"
)

func TestCompress_ArrayTruncation(t *testing.T) {
	t.Parallel()
	c := compression.New(compression.Config{MaxArrayLen: 3, MaxDepth: 10, MaxStringLen: 2000})

	items := make([]any, 50)
	for i := range items {
		items[i] = map[string]any{"id": i, "name": "item"}
	}
	data, _ := json.Marshal(map[string]any{"results": items})

	result := c.Compress(data)
	var out map[string]any
	json.Unmarshal(result, &out)

	wrapper := out["results"].(map[string]any)
	if wrapper["_truncated"] != true {
		t.Error("expected truncation marker")
	}
	if wrapper["_total"] != float64(50) {
		t.Errorf("_total=%v, want 50", wrapper["_total"])
	}
	if wrapper["_shown"] != float64(3) {
		t.Errorf("_shown=%v, want 3", wrapper["_shown"])
	}
	shown := wrapper["_items"].([]any)
	if len(shown) != 3 {
		t.Errorf("items=%d, want 3", len(shown))
	}
}

func TestCompress_DepthLimit(t *testing.T) {
	t.Parallel()
	c := compression.New(compression.Config{MaxArrayLen: 20, MaxDepth: 2, MaxStringLen: 2000})

	input := `{"a":{"b":{"c":{"d":"too deep"}}}}`
	result := c.Compress([]byte(input))

	if !strings.Contains(string(result), "nested beyond depth limit") {
		t.Errorf("expected depth limit message, got: %s", result)
	}
}

func TestCompress_StringTruncation(t *testing.T) {
	t.Parallel()
	c := compression.New(compression.Config{MaxArrayLen: 20, MaxDepth: 10, MaxStringLen: 50})

	longStr := strings.Repeat("x", 500)
	data, _ := json.Marshal(map[string]any{"content": longStr})

	result := c.Compress(data)
	var out map[string]any
	json.Unmarshal(result, &out)

	content := out["content"].(string)
	if len(content) > 100 {
		t.Errorf("content should be truncated, len=%d", len(content))
	}
	if !strings.Contains(content, "truncated") {
		t.Error("should contain truncation notice")
	}
	if !strings.Contains(content, "500 total chars") {
		t.Error("should contain original length")
	}
}

func TestCompress_PreservesSmallData(t *testing.T) {
	t.Parallel()
	c := compression.New(compression.DefaultConfig())

	input := `{"name":"test","count":42,"active":true}`
	result := c.Compress([]byte(input))

	var original, compressed map[string]any
	json.Unmarshal([]byte(input), &original)
	json.Unmarshal(result, &compressed)

	if compressed["name"] != original["name"] ||
		compressed["count"] != original["count"] ||
		compressed["active"] != original["active"] {
		t.Errorf("small data should be preserved: got %s", result)
	}
}

func TestCompress_InvalidJSON(t *testing.T) {
	t.Parallel()
	c := compression.New(compression.DefaultConfig())
	bad := []byte("not json {{{")
	result := c.Compress(bad)
	if string(result) != string(bad) {
		t.Error("invalid JSON should be returned unchanged")
	}
}

func TestCompress_NestedArrays(t *testing.T) {
	t.Parallel()
	c := compression.New(compression.Config{MaxArrayLen: 2, MaxDepth: 10, MaxStringLen: 2000})

	input := `{"outer":[["a","b","c","d"],["e","f","g","h"],["i","j"]]}`
	result := c.Compress([]byte(input))

	var out map[string]any
	json.Unmarshal(result, &out)
	wrapper := out["outer"].(map[string]any)
	if wrapper["_truncated"] != true {
		t.Error("outer array should be truncated")
	}
}

func TestRatio(t *testing.T) {
	t.Parallel()
	original := []byte(`{"data":"` + strings.Repeat("x", 1000) + `"}`)
	compressed := []byte(`{"data":"truncated"}`)

	r := compression.Ratio(original, compressed)
	if r < 0.9 {
		t.Errorf("ratio=%.2f, expected >0.9", r)
	}
}

func TestRatio_EmptyOriginal(t *testing.T) {
	t.Parallel()
	if compression.Ratio(nil, []byte("x")) != 0 {
		t.Error("empty original should return 0")
	}
}

func TestDefaultConfig(t *testing.T) {
	t.Parallel()
	cfg := compression.DefaultConfig()
	if cfg.MaxArrayLen != 20 {
		t.Errorf("MaxArrayLen=%d, want 20", cfg.MaxArrayLen)
	}
	if cfg.MaxDepth != 8 {
		t.Errorf("MaxDepth=%d, want 8", cfg.MaxDepth)
	}
	if cfg.MaxStringLen != 2000 {
		t.Errorf("MaxStringLen=%d, want 2000", cfg.MaxStringLen)
	}
}
