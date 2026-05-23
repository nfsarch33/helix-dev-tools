// runx-public-repo-gate: allow-file secret_cred_ref,personal_path_id — zdproxy targets the Zendesk AI gateway and requires the literal gateway URL plus token env-var name

// Package zdproxy implements a MacBook-local Anthropic Messages translator that
// routes Claude Code CLI / Desktop Code-runtime children to the Zendesk AI
// gateway (Bedrock and OpenAI surfaces).
//
// Hard constraints (enforced at boot):
//   - listener binds only to a loopback address
//   - metrics listener binds only to a loopback address
//   - gateway base URLs must be https://
//   - upstream tokens are resolved from 1Password at startup and never persisted
//
// This package is MacBook-only by repo-owner directive (v256 D28). It must not
// be wired into the home fleet (llm-cluster-router, gstack research-agent,
// pdf-mcp-server, Helixon runtime, mission-control, member DevOps/SysAdmin
// agents, Hermes, OpenClaw, or any Tailscale/OCI-attached node).
package zdproxy

import (
	"fmt"
	"net"
	"strings"
)

// Config carries the proxy's runtime configuration. All fields are populated
// from CLI flags; nothing reads or writes the filesystem (other than the local
// auth-token file the server creates at startup).
type Config struct {
	// Bind is the host:port for the inbound Anthropic-Messages listener.
	// Must resolve to a loopback address (127.0.0.0/8 or ::1).
	Bind string

	// MetricsBind is the host:port for the Prometheus metrics listener.
	// Must resolve to a loopback address.
	MetricsBind string

	// BedrockBaseURL is the ZD Bedrock gateway base URL. Must be https://.
	BedrockBaseURL string

	// OpenAIBaseURL is the ZD OpenAI Chat Completions / Responses gateway base
	// URL. Must be https://.
	OpenAIBaseURL string

	// OpBedrockItem is the 1Password item title carrying the ZD Bedrock bearer
	// in `notesPlain` (resolved by package secrets at startup).
	OpBedrockItem string

	// OpBedrockField is the shell env-var name embedded in the bedrock item's
	// notesPlain snippet (e.g. AWS_BEARER_TOKEN_BEDROCK). When empty the
	// secrets loader treats the entire notesPlain as the bearer (legacy
	// shape); when set it parses `export FIELD=VALUE` and returns VALUE.
	OpBedrockField string

	// OpOpenAIItem is the 1Password item title carrying the ZD OpenAI bearer
	// in `notesPlain` (resolved by package secrets at startup).
	OpOpenAIItem string

	// OpOpenAIField behaves the same as OpBedrockField for the OpenAI item
	// (e.g. OPENAI_API_KEY).
	OpOpenAIField string

	// LocalTokenPath, if non-empty, overrides the default
	// `${XDG_CONFIG_HOME:-$HOME/.config}/zd-claude-proxy/local-token` file
	// where the per-process local auth token is written (mode 0600).
	LocalTokenPath string

	// VPNHostnames lists hostnames that must resolve before the proxy starts
	// taking traffic. If any DNS lookup fails the proxy refuses to start so
	// callers don't accidentally emit Anthropic-key-shaped traffic to a public
	// resolver while the corp VPN is off.
	VPNHostnames []string
}

// Validate enforces the boot-time invariants. Returns a non-nil error if any
// invariant is violated; the caller must abort startup in that case.
func (c Config) Validate() error {
	if err := validateLoopbackBind(c.Bind, "bind"); err != nil {
		return err
	}
	if err := validateLoopbackBind(c.MetricsBind, "metrics bind"); err != nil {
		return err
	}
	if c.BedrockBaseURL == "" {
		return fmt.Errorf("bedrock base url required")
	}
	if !strings.HasPrefix(c.BedrockBaseURL, "https://") {
		return fmt.Errorf("bedrock base url must use https://, got %q", c.BedrockBaseURL)
	}
	if c.OpenAIBaseURL == "" {
		return fmt.Errorf("openai base url required")
	}
	if !strings.HasPrefix(c.OpenAIBaseURL, "https://") {
		return fmt.Errorf("openai base url must use https://, got %q", c.OpenAIBaseURL)
	}
	if c.OpBedrockItem == "" {
		return fmt.Errorf("op bedrock item title required")
	}
	if c.OpOpenAIItem == "" {
		return fmt.Errorf("op openai item title required")
	}
	return nil
}

func validateLoopbackBind(bind, label string) error {
	if bind == "" {
		return fmt.Errorf("%s must bind to a loopback address (got empty)", label)
	}
	host, port, err := net.SplitHostPort(bind)
	if err != nil {
		return fmt.Errorf("%s expected host:port form, got %q: %w", label, bind, err)
	}
	if port == "" {
		return fmt.Errorf("%s expected host:port form, missing port in %q", label, bind)
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		return fmt.Errorf("%s must bind to a loopback address, got %q", label, host)
	}
	if isLoopbackHost(host) {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return nil
	}
	if label == "metrics bind" {
		return fmt.Errorf("metrics bind must be loopback, got %q", host)
	}
	return fmt.Errorf("%s must bind to a loopback address, got %q", label, host)
}

func isLoopbackHost(host string) bool {
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1":
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}
