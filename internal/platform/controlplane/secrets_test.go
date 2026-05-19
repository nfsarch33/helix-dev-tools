package controlplane

import (
	"os"
	"testing"
)

func TestSecretResolver_FromEnv(t *testing.T) {
	os.Setenv("TEST_SECRET_XYZ", "secret_value")
	defer os.Unsetenv("TEST_SECRET_XYZ")

	r := NewSecretResolver("test-service")
	val, err := r.Resolve("TEST_SECRET_XYZ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "secret_value" {
		t.Errorf("expected secret_value, got %s", val)
	}
}

func TestSecretResolver_NotFound(t *testing.T) {
	r := NewSecretResolver("nonexistent-service")
	_, err := r.Resolve("DEFINITELY_NOT_SET_ABC123")
	if err == nil {
		t.Error("expected error for missing secret")
	}
}

func TestSecretResolver_Cache(t *testing.T) {
	os.Setenv("CACHE_TEST_KEY", "cached")
	defer os.Unsetenv("CACHE_TEST_KEY")

	r := NewSecretResolver("test")
	r.Resolve("CACHE_TEST_KEY")

	if r.CachedCount() != 1 {
		t.Errorf("expected 1 cached, got %d", r.CachedCount())
	}

	r.ClearCache()
	if r.CachedCount() != 0 {
		t.Error("expected empty cache after clear")
	}
}

func TestSecretResolver_CacheHit(t *testing.T) {
	os.Setenv("HIT_KEY", "first")
	defer os.Unsetenv("HIT_KEY")

	r := NewSecretResolver("test")
	r.Resolve("HIT_KEY")

	os.Setenv("HIT_KEY", "second")
	val, _ := r.Resolve("HIT_KEY")
	if val != "first" {
		t.Error("expected cached value, not fresh env read")
	}
}
