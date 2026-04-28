package zdproxy

import (
	"context"
	"errors"
	"testing"
)

type fakeOpResolver struct {
	items map[string]string
	err   error
}

func (f *fakeOpResolver) Resolve(_ context.Context, item, field string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	v, ok := f.items[item+"::"+field]
	if !ok {
		return "", errors.New("not found")
	}
	return v, nil
}

func TestLoadSecrets_PopulatesBothBearers(t *testing.T) {
	r := &fakeOpResolver{
		items: map[string]string{
			"zd api gateway bedrock claude models::notesPlain": "zdai_bedrock_BEARERVALUE",
			"zd api gateway openai models::notesPlain":         "zdai_openai_BEARERVALUE",
		},
	}
	cfg := Config{
		OpBedrockItem: "zd api gateway bedrock claude models",
		OpOpenAIItem:  "zd api gateway openai models",
	}
	s, err := LoadSecrets(context.Background(), cfg, r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.BedrockBearer != "zdai_bedrock_BEARERVALUE" {
		t.Fatalf("bedrock bearer mismatch: %q", redact(s.BedrockBearer))
	}
	if s.OpenAIBearer != "zdai_openai_BEARERVALUE" {
		t.Fatalf("openai bearer mismatch: %q", redact(s.OpenAIBearer))
	}
}

func TestLoadSecrets_FailsFastOnMissing(t *testing.T) {
	r := &fakeOpResolver{items: map[string]string{}}
	cfg := Config{
		OpBedrockItem: "zd api gateway bedrock claude models",
		OpOpenAIItem:  "zd api gateway openai models",
	}
	_, err := LoadSecrets(context.Background(), cfg, r)
	if err == nil {
		t.Fatal("expected error on missing bedrock item, got nil")
	}
}

func TestRedact_NeverEchoesValue(t *testing.T) {
	got := redact("super-secret-zdai-bearer-value-12345")
	if got == "super-secret-zdai-bearer-value-12345" {
		t.Fatal("redact must not return the original value")
	}
	if got == "" {
		t.Fatal("redact must produce a non-empty placeholder")
	}
	// Should mention length only, not the value.
	if want := "<redacted len=36>"; got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

// TestLoadSecrets_ExtractsFromShellSnippet is the v256.5 hotfix regression
// test: the actual `zd api gateway bedrock claude models` and `zd api gateway
// openai models` items in 1Password store a multi-line shell snippet in
// notesPlain (export AWS_BEARER_TOKEN_BEDROCK=..., export OPENAI_API_KEY=...).
// LoadSecrets must extract the value, not return the whole snippet.
func TestLoadSecrets_ExtractsFromShellSnippet(t *testing.T) {
	bedrockNotes := `"export AWS_ENDPOINT_URL_BEDROCK_RUNTIME=https://ai-gateway.zende.sk/bedrock
export AWS_BEARER_TOKEN_BEDROCK=zdai_BEDROCK_VALUE_FROM_OP

curl ${AWS_ENDPOINT_URL_BEDROCK_RUNTIME}/model/foo/invoke ..."`
	openaiNotes := `"export OPENAI_BASE_URL=https://ai-gateway.zende.sk/v1
export OPENAI_API_KEY=zdai_OPENAI_VALUE_FROM_OP

curl ${OPENAI_BASE_URL}/responses ..."`
	r := &fakeOpResolver{
		items: map[string]string{
			"zd api gateway bedrock claude models::notesPlain": bedrockNotes,
			"zd api gateway openai models::notesPlain":         openaiNotes,
		},
	}
	cfg := Config{
		OpBedrockItem:  "zd api gateway bedrock claude models",
		OpBedrockField: "AWS_BEARER_TOKEN_BEDROCK",
		OpOpenAIItem:   "zd api gateway openai models",
		OpOpenAIField:  "OPENAI_API_KEY",
	}
	s, err := LoadSecrets(context.Background(), cfg, r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.BedrockBearer != "zdai_BEDROCK_VALUE_FROM_OP" {
		t.Fatalf("bedrock bearer mismatch: got len=%d", len(s.BedrockBearer))
	}
	if s.OpenAIBearer != "zdai_OPENAI_VALUE_FROM_OP" {
		t.Fatalf("openai bearer mismatch: got len=%d", len(s.OpenAIBearer))
	}
}

func TestLoadSecrets_ExtractFails_WhenFieldAbsent(t *testing.T) {
	r := &fakeOpResolver{
		items: map[string]string{
			"bedrock-item::notesPlain": "no exports in here",
			"openai-item::notesPlain":  "export OPENAI_API_KEY=ok",
		},
	}
	cfg := Config{
		OpBedrockItem:  "bedrock-item",
		OpBedrockField: "AWS_BEARER_TOKEN_BEDROCK",
		OpOpenAIItem:   "openai-item",
		OpOpenAIField:  "OPENAI_API_KEY",
	}
	_, err := LoadSecrets(context.Background(), cfg, r)
	if err == nil {
		t.Fatal("expected error when bedrock export missing, got nil")
	}
}

func TestExtractBearer_QuoteHandling(t *testing.T) {
	cases := []struct {
		name    string
		snippet string
		field   string
		want    string
	}{
		{"plain", "export FOO=BAR", "FOO", "BAR"},
		{"leading-quote", `"export FOO=BAR`, "FOO", "BAR"},
		{"trailing-quote", `export FOO=BAR"`, "FOO", "BAR"},
		{"single-quote-value", `export FOO='BAR'`, "FOO", "BAR"},
		{"double-quote-value", `export FOO="BAR"`, "FOO", "BAR"},
		{"multi-line", "comment\nexport AWS_BEARER_TOKEN_BEDROCK=zdai_X\n", "AWS_BEARER_TOKEN_BEDROCK", "zdai_X"},
		{"legacy-empty-field", "raw-bearer-value", "", "raw-bearer-value"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := extractBearer(tc.snippet, tc.field)
			if err != nil {
				t.Fatalf("extractBearer: %v", err)
			}
			if got != tc.want {
				t.Fatalf("want=%q got=%q", tc.want, got)
			}
		})
	}
}
