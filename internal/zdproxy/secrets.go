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
//
// When cfg.OpBedrockField is non-empty, the loader parses the bedrock item's
// notesPlain shell snippet for `export <field>=<value>` and uses VALUE as the
// bearer. Same for cfg.OpOpenAIField with the openai item. When the field
// names are empty the loader uses the legacy "whole notesPlain == bearer"
// shape for backwards compatibility with the original `ZD Claude Code AI
// Gateway bedrock` item.
func LoadSecrets(ctx context.Context, cfg Config, r OpResolver) (Secrets, error) {
	bedrockNotes, err := r.Resolve(ctx, cfg.OpBedrockItem, "notesPlain")
	if err != nil {
		return Secrets{}, fmt.Errorf("resolve bedrock bearer from %q: %w", cfg.OpBedrockItem, err)
	}
	bedrockBearer, err := extractBearer(bedrockNotes, cfg.OpBedrockField)
	if err != nil {
		return Secrets{}, fmt.Errorf("extract bedrock bearer from %q: %w", cfg.OpBedrockItem, err)
	}

	openaiNotes, err := r.Resolve(ctx, cfg.OpOpenAIItem, "notesPlain")
	if err != nil {
		return Secrets{}, fmt.Errorf("resolve openai bearer from %q: %w", cfg.OpOpenAIItem, err)
	}
	openaiBearer, err := extractBearer(openaiNotes, cfg.OpOpenAIField)
	if err != nil {
		return Secrets{}, fmt.Errorf("extract openai bearer from %q: %w", cfg.OpOpenAIItem, err)
	}
	return Secrets{
		BedrockBearer: bedrockBearer,
		OpenAIBearer:  openaiBearer,
	}, nil
}

// extractBearer returns the bearer from a 1Password notesPlain snippet.
//
// If field is empty, the entire (trimmed) snippet is treated as the bearer.
// If field is non-empty, the snippet is scanned for a line of the form
// `export <field>=<value>` and the trimmed VALUE is returned. Surrounding
// outer double-quotes (added by the op CLI when rendering multi-line notes)
// are tolerated.
func extractBearer(snippet, field string) (string, error) {
	if strings.TrimSpace(snippet) == "" {
		return "", fmt.Errorf("empty notesPlain")
	}
	if field == "" {
		return strings.TrimSpace(snippet), nil
	}
	needle := "export " + field + "="
	for _, raw := range strings.Split(snippet, "\n") {
		line := strings.TrimSpace(raw)
		line = strings.TrimPrefix(line, `"`)
		line = strings.TrimSuffix(line, `"`)
		if !strings.HasPrefix(line, needle) {
			continue
		}
		v := strings.TrimSpace(line[len(needle):])
		v = strings.Trim(v, `'"`)
		if v == "" {
			return "", fmt.Errorf("export %s=<empty>", field)
		}
		return v, nil
	}
	return "", fmt.Errorf("no `export %s=...` line in notesPlain", field)
}

// redact returns a length-only placeholder for sensitive strings; never the
// value itself.
func redact(v string) string {
	return fmt.Sprintf("<redacted len=%d>", len(v))
}
