// runx-public-repo-gate: allow-file secret_cred_ref,personal_path_id,network_topology — tests verify zdproxy gate against literal gateway URL, env-var, and Tailscale IP samples

package zdproxy

import (
	"strings"
	"testing"
)

func TestConfig_Validate_LoopbackOnly(t *testing.T) {
	cases := []struct {
		name    string
		bind    string
		wantErr string
	}{
		{"loopback ipv4 ok", "127.0.0.1:8767", ""},
		{"loopback ipv6 ok", "[::1]:8767", ""},
		{"localhost ok", "localhost:8767", ""},
		{"all-interfaces rejected", "0.0.0.0:8767", "must bind to a loopback"},
		{"empty rejected", "", "must bind to a loopback"},
		{"lan address rejected", "100.64.1.5:8767", "must bind to a loopback"},
		{"lan address ipv4 rejected", "192.168.1.10:8767", "must bind to a loopback"},
		{"missing port rejected", "127.0.0.1", "expected host:port"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{
				Bind:           tc.bind,
				MetricsBind:    "127.0.0.1:9787",
				BedrockBaseURL: "https://ai-gateway.zende.sk/bedrock",
				OpenAIBaseURL:  "https://ai-gateway.zende.sk/v1",
				OpBedrockItem:  "zd api gateway bedrock claude models",
				OpOpenAIItem:   "zd api gateway openai models",
			}
			err := cfg.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error to contain %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}

func TestConfig_Validate_MetricsBindLoopback(t *testing.T) {
	cfg := Config{
		Bind:           "127.0.0.1:8767",
		MetricsBind:    "0.0.0.0:9787",
		BedrockBaseURL: "https://ai-gateway.zende.sk/bedrock",
		OpenAIBaseURL:  "https://ai-gateway.zende.sk/v1",
		OpBedrockItem:  "zd api gateway bedrock claude models",
		OpOpenAIItem:   "zd api gateway openai models",
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatalf("expected loopback-only metrics error, got nil")
	}
	if !strings.Contains(err.Error(), "metrics bind") || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("expected metrics-bind loopback error, got %v", err)
	}
}

func TestConfig_Validate_GatewayURLs(t *testing.T) {
	cases := []struct {
		name    string
		bedrock string
		openai  string
		wantErr string
	}{
		{"http bedrock rejected", "http://ai-gateway.zende.sk/bedrock", "https://ai-gateway.zende.sk/v1", "must use https"},
		{"http openai rejected", "https://ai-gateway.zende.sk/bedrock", "http://ai-gateway.zende.sk/v1", "must use https"},
		{"empty bedrock rejected", "", "https://ai-gateway.zende.sk/v1", "bedrock base url required"},
		{"empty openai rejected", "https://ai-gateway.zende.sk/bedrock", "", "openai base url required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{
				Bind:           "127.0.0.1:8767",
				MetricsBind:    "127.0.0.1:9787",
				BedrockBaseURL: tc.bedrock,
				OpenAIBaseURL:  tc.openai,
				OpBedrockItem:  "zd api gateway bedrock claude models",
				OpOpenAIItem:   "zd api gateway openai models",
			}
			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error %q, got %v", tc.wantErr, err)
			}
		})
	}
}
