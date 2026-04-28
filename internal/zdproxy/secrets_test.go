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
