package replicate

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFilterMCP_EmptyInput(t *testing.T) {
	t.Parallel()
	got, err := FilterMCP(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("empty input must yield nil, got %q", string(got))
	}
}

func TestFilterMCP_DropsTestMCPAndDisabled(t *testing.T) {
	t.Parallel()
	in := []byte(`{
		"mcpServers": {
			"test-mcp": {"command": "python3"},
			"context7": {"command": "npx", "args": ["-y", "@upstash/context7-mcp"]},
			"memory":   {"command": "npx", "disabled": true},
			"git-mcp":  {"command": "npx", "disabled": false}
		}
	}`)
	out, err := FilterMCP(in)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		MCPServers map[string]map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("output not valid JSON: %v\noutput=%s", err, string(out))
	}
	if _, present := got.MCPServers["test-mcp"]; present {
		t.Errorf("test-mcp must be dropped, got %v", got.MCPServers)
	}
	if _, present := got.MCPServers["memory"]; present {
		t.Errorf("disabled=true server must be dropped, got %v", got.MCPServers)
	}
	if _, present := got.MCPServers["context7"]; !present {
		t.Errorf("context7 must survive, got %v", got.MCPServers)
	}
	if _, present := got.MCPServers["git-mcp"]; !present {
		t.Errorf("disabled=false survivor must keep server, got %v", got.MCPServers)
	}
	if v, present := got.MCPServers["git-mcp"]["disabled"]; present {
		t.Errorf("disabled key must be stripped from survivors, got %v", v)
	}
}

func TestFilterMCP_DeterministicOrder(t *testing.T) {
	t.Parallel()
	in := []byte(`{"mcpServers":{"zulu":{"command":"a"},"alpha":{"command":"b"},"mike":{"command":"c"}}}`)
	out1, _ := FilterMCP(in)
	out2, _ := FilterMCP(in)
	if string(out1) != string(out2) {
		t.Fatalf("FilterMCP must be deterministic across runs:\n%s\n%s", out1, out2)
	}
	// Check alpha appears before mike appears before zulu in the
	// serialised string. encoding/json sorts maps by key.
	s := string(out1)
	if strings.Index(s, "alpha") > strings.Index(s, "mike") {
		t.Errorf("alpha must precede mike: %s", s)
	}
	if strings.Index(s, "mike") > strings.Index(s, "zulu") {
		t.Errorf("mike must precede zulu: %s", s)
	}
}

func TestFilterMCP_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, err := FilterMCP([]byte("not json"))
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	if !strings.Contains(err.Error(), "parse cursor mcp.json") {
		t.Fatalf("error message should include context, got %q", err.Error())
	}
}
