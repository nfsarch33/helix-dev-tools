package zdproxy

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Secrets carries the upstream bearer tokens resolved at startup. They are
// never persisted to disk, never logged in full, and only ever rendered via
// `redact` when surfaced.
type Secrets struct {
	BedrockBearer string
	OpenAIBearer  string
}

// OpResolver abstracts the 1Password lookup so the production code can call
// the `op` CLI while tests can swap in a fake.
type OpResolver interface {
	// Resolve returns the value of `field` (e.g. "notesPlain", "credential")
	// on the named item. Implementations must NOT log the returned value.
	Resolve(ctx context.Context, item, field string) (string, error)
}

// CLIOpResolver shells out to the real `op` CLI (1Password). Each call invokes
// `op item get <item> --fields <field> --reveal`. Stdout is parsed to extract
// the requested field.
type CLIOpResolver struct {
	// Vault, if non-empty, scopes the lookup to a specific vault.
	Vault string
}

// Resolve implements OpResolver against the 1Password CLI.
func (r *CLIOpResolver) Resolve(ctx context.Context, item, field string) (string, error) {
	args := []string{"item", "get", item, "--fields", field, "--reveal"}
	if r.Vault != "" {
		args = append(args, "--vault", r.Vault)
	}
	cmd := exec.CommandContext(ctx, "op", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("op item get %q field=%q: %w (stderr: %s)", item, field, err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimRight(stdout.String(), "\n"), nil
}

// LoadSecrets resolves the gateway bearer tokens from 1Password and returns a
// populated Secrets. Failure to resolve any bearer is fatal — the proxy must
// never start partially configured.
func LoadSecrets(ctx context.Context, cfg Config, r OpResolver) (Secrets, error) {
	bedrock, err := r.Resolve(ctx, cfg.OpBedrockItem, "notesPlain")
	if err != nil {
		return Secrets{}, fmt.Errorf("resolve bedrock bearer from %q: %w", cfg.OpBedrockItem, err)
	}
	if strings.TrimSpace(bedrock) == "" {
		return Secrets{}, fmt.Errorf("resolve bedrock bearer from %q: empty value", cfg.OpBedrockItem)
	}
	openai, err := r.Resolve(ctx, cfg.OpOpenAIItem, "notesPlain")
	if err != nil {
		return Secrets{}, fmt.Errorf("resolve openai bearer from %q: %w", cfg.OpOpenAIItem, err)
	}
	if strings.TrimSpace(openai) == "" {
		return Secrets{}, fmt.Errorf("resolve openai bearer from %q: empty value", cfg.OpOpenAIItem)
	}
	return Secrets{
		BedrockBearer: strings.TrimSpace(bedrock),
		OpenAIBearer:  strings.TrimSpace(openai),
	}, nil
}

// redact returns a length-only placeholder for sensitive strings; never the
// value itself.
func redact(v string) string {
	return fmt.Sprintf("<redacted len=%d>", len(v))
}
