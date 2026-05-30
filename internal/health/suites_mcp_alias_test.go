package health

import "testing"

func TestResolveMCPServer_prefersFirstMatch(t *testing.T) {
	cfg := &mcpHealthConfig{
		MCPServers: map[string]mcpHealthServerSpec{
			"user-engram": {Command: "/home/jason/bin/engram"},
			"mem0":        {Command: "uvx"},
		},
	}

	spec, name, ok := resolveMCPServer(cfg, "user-engram", "mem0")
	if !ok {
		t.Fatal("expected match")
	}
	if name != "user-engram" {
		t.Fatalf("name = %q, want user-engram", name)
	}
	if spec.Command != "/home/jason/bin/engram" {
		t.Fatalf("command = %q", spec.Command)
	}
}

func TestResolveMCPServer_legacyFallback(t *testing.T) {
	cfg := &mcpHealthConfig{
		MCPServers: map[string]mcpHealthServerSpec{
			"context-mode": {Command: "/bin/context-mode"},
		},
	}

	_, name, ok := resolveMCPServer(cfg, "user-context-mode", "context-mode")
	if !ok || name != "context-mode" {
		t.Fatalf("got ok=%v name=%q", ok, name)
	}
}

func TestResolveMCPServer_perplexityAliases(t *testing.T) {
	cfg := &mcpHealthConfig{
		MCPServers: map[string]mcpHealthServerSpec{
			"user-perplexity-ask": {Command: "npx"},
		},
	}

	_, name, ok := resolveMCPServer(cfg, "perplexity-ask", "perplexity", "user-perplexity-ask")
	if !ok || name != "user-perplexity-ask" {
		t.Fatalf("got ok=%v name=%q", ok, name)
	}
}

func TestResolveMCPServer_missing(t *testing.T) {
	cfg := &mcpHealthConfig{MCPServers: map[string]mcpHealthServerSpec{}}
	_, _, ok := resolveMCPServer(cfg, "mem0", "user-engram")
	if ok {
		t.Fatal("expected no match")
	}
}
