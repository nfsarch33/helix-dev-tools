package replicate

import (
	"encoding/json"
	"fmt"
	"sort"
)

// FilterMCP rewrites a Cursor mcp.json blob into a Claude-Code-friendly
// shape:
//
//   - drops any server with `"disabled": true`
//   - drops the cursor-only `test-mcp` placeholder
//   - drops the `disabled` key from surviving entries (Claude Code does
//     not understand it and warns on unknown keys)
//   - keeps the rest of the server config verbatim (command, args, env)
//
// The output keeps the `{"mcpServers": {...}}` envelope so Claude Code
// reads it without further translation. The output is canonicalised by
// sorting servers alphabetically — this keeps the rewritten file stable
// under repeated runs (important for the dry-run idempotency check).
func FilterMCP(raw []byte) ([]byte, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var doc struct {
		MCPServers map[string]map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse cursor mcp.json: %w", err)
	}
	out := struct {
		MCPServers map[string]map[string]any `json:"mcpServers"`
	}{
		MCPServers: map[string]map[string]any{},
	}
	for name, server := range doc.MCPServers {
		if name == "test-mcp" {
			continue
		}
		if v, ok := server["disabled"]; ok {
			if b, isBool := v.(bool); isBool && b {
				continue
			}
			delete(server, "disabled")
		}
		out.MCPServers[name] = server
	}
	// Canonical key order: sort by name. encoding/json marshals maps in
	// sorted-key order so this is technically already deterministic on
	// the wire, but we keep the explicit sort to make the intent
	// readable and to guard against future runtime changes.
	names := make([]string, 0, len(out.MCPServers))
	for k := range out.MCPServers {
		names = append(names, k)
	}
	sort.Strings(names)
	canonical := struct {
		MCPServers map[string]map[string]any `json:"mcpServers"`
	}{MCPServers: map[string]map[string]any{}}
	for _, n := range names {
		canonical.MCPServers[n] = out.MCPServers[n]
	}
	b, err := json.MarshalIndent(canonical, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal filtered mcp.json: %w", err)
	}
	return append(b, '\n'), nil
}
