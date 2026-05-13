package compression

import (
	"encoding/json"
	"fmt"
)

// Config controls compression behaviour.
type Config struct {
	MaxArrayLen  int `json:"max_array_len"`
	MaxDepth     int `json:"max_depth"`
	MaxStringLen int `json:"max_string_len"`
}

// DefaultConfig returns conservative compression defaults.
func DefaultConfig() Config {
	return Config{
		MaxArrayLen:  20,
		MaxDepth:     8,
		MaxStringLen: 2000,
	}
}

// Compressor reduces JSON response size for LLM context windows.
type Compressor struct {
	cfg Config
}

// New creates a Compressor with the given config.
func New(cfg Config) *Compressor {
	if cfg.MaxArrayLen <= 0 {
		cfg.MaxArrayLen = 20
	}
	if cfg.MaxDepth <= 0 {
		cfg.MaxDepth = 8
	}
	if cfg.MaxStringLen <= 0 {
		cfg.MaxStringLen = 2000
	}
	return &Compressor{cfg: cfg}
}

// Compress takes raw JSON and returns a compressed version.
// Returns input unchanged if parsing fails.
func (c *Compressor) Compress(data []byte) []byte {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return data
	}
	compressed := c.compressValue(v, 0)
	out, err := json.Marshal(compressed)
	if err != nil {
		return data
	}
	return out
}

func (c *Compressor) compressValue(v any, depth int) any {
	if depth > c.cfg.MaxDepth {
		return "[...nested beyond depth limit]"
	}

	switch val := v.(type) {
	case map[string]any:
		result := make(map[string]any, len(val))
		for k, child := range val {
			result[k] = c.compressValue(child, depth+1)
		}
		return result

	case []any:
		if len(val) > c.cfg.MaxArrayLen {
			truncated := make([]any, c.cfg.MaxArrayLen)
			for i := 0; i < c.cfg.MaxArrayLen; i++ {
				truncated[i] = c.compressValue(val[i], depth+1)
			}
			return map[string]any{
				"_items":     truncated,
				"_truncated": true,
				"_total":     len(val),
				"_shown":     c.cfg.MaxArrayLen,
				"_hint":      "use offset/limit to paginate",
			}
		}
		result := make([]any, len(val))
		for i, child := range val {
			result[i] = c.compressValue(child, depth+1)
		}
		return result

	case string:
		if len(val) > c.cfg.MaxStringLen {
			return val[:c.cfg.MaxStringLen] + fmt.Sprintf("...[truncated, %d total chars]", len(val))
		}
		return val

	default:
		return v
	}
}

// Ratio returns the compression ratio (0.0 = identical, 1.0 = fully removed).
func Ratio(original, compressed []byte) float64 {
	if len(original) == 0 {
		return 0
	}
	return 1 - float64(len(compressed))/float64(len(original))
}
