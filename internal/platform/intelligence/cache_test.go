package intelligence

import (
	"testing"
	"time"
)

func TestResponseCache_SetAndGet(t *testing.T) {
	c := NewResponseCache(10, 5*time.Minute)
	c.Set("hello", "world")

	val, ok := c.Get("hello")
	if !ok || val != "world" {
		t.Errorf("expected world, got %v", val)
	}
}

func TestResponseCache_Miss(t *testing.T) {
	c := NewResponseCache(10, 5*time.Minute)
	_, ok := c.Get("nonexistent")
	if ok {
		t.Error("expected cache miss")
	}
}

func TestResponseCache_TTLExpiry(t *testing.T) {
	c := NewResponseCache(10, 1*time.Millisecond)
	c.Set("key", "value")
	time.Sleep(5 * time.Millisecond)

	_, ok := c.Get("key")
	if ok {
		t.Error("expected cache miss after TTL expiry")
	}
}

func TestResponseCache_Eviction(t *testing.T) {
	c := NewResponseCache(2, 1*time.Hour)
	c.Set("a", "1")
	c.Set("b", "2")
	c.Set("c", "3")

	if c.Size() != 2 {
		t.Errorf("expected size 2 after eviction, got %d", c.Size())
	}
}

func TestResponseCache_Clear(t *testing.T) {
	c := NewResponseCache(10, 1*time.Hour)
	c.Set("x", "y")
	c.Clear()

	if c.Size() != 0 {
		t.Error("expected empty after clear")
	}
}

func TestResponseCache_SemanticDedup(t *testing.T) {
	c := NewResponseCache(10, 1*time.Hour)
	c.Set("exact same prompt", "response1")

	val, ok := c.Get("exact same prompt")
	if !ok || val != "response1" {
		t.Error("same prompt should return cached response")
	}
}
