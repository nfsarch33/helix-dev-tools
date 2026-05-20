package providerswitch

import (
	"testing"
)

func testProviders() []Provider {
	return []Provider{
		{Name: "minimax", BaseURL: "https://api.minimaxi.com/v1", Model: "MiniMax-M2.7-highspeed", Priority: 10},
		{Name: "perplexity", BaseURL: "https://api.perplexity.ai/v1", Model: "pplx-embed-v1", Priority: 5},
		{Name: "local", BaseURL: "http://localhost:8787/v1", Model: "Qwen3.5-4B", Priority: 1},
	}
}

func TestActiveSelectsHighestPriority(t *testing.T) {
	r := New(testProviders())
	active := r.Active()
	if active.Name != "minimax" {
		t.Errorf("expected minimax (priority 10), got %s", active.Name)
	}
}

func TestMarkUnhealthyFailsOver(t *testing.T) {
	r := New(testProviders())
	r.MarkUnhealthy("minimax")
	active := r.Active()
	if active.Name != "perplexity" {
		t.Errorf("expected perplexity after minimax fail, got %s", active.Name)
	}
}

func TestMarkHealthyRestores(t *testing.T) {
	r := New(testProviders())
	r.MarkUnhealthy("minimax")
	r.MarkHealthy("minimax")
	active := r.Active()
	if active.Name != "minimax" {
		t.Errorf("expected minimax after restore, got %s", active.Name)
	}
}

func TestFailoverChain(t *testing.T) {
	r := New(testProviders())
	r.Failover()
	if r.Active().Name != "perplexity" {
		t.Errorf("first failover: %s", r.Active().Name)
	}
	r.Failover()
	if r.Active().Name != "local" {
		t.Errorf("second failover: %s", r.Active().Name)
	}
	err := r.Failover()
	if err == nil {
		t.Fatal("third failover should error (no healthy)")
	}
}

func TestHealthyCount(t *testing.T) {
	r := New(testProviders())
	if r.HealthyCount() != 3 {
		t.Errorf("expected 3, got %d", r.HealthyCount())
	}
	r.MarkUnhealthy("minimax")
	if r.HealthyCount() != 2 {
		t.Errorf("expected 2, got %d", r.HealthyCount())
	}
}

func TestList(t *testing.T) {
	r := New(testProviders())
	list := r.List()
	if len(list) != 3 {
		t.Errorf("expected 3 providers, got %d", len(list))
	}
}

func TestEmptyRouter(t *testing.T) {
	r := New(nil)
	active := r.Active()
	if active.Name != "" {
		t.Errorf("expected empty provider, got %s", active.Name)
	}
}

func TestFailoverWithSingleProvider(t *testing.T) {
	r := New([]Provider{{Name: "only", Priority: 1}})
	err := r.Failover()
	if err == nil {
		t.Fatal("single provider failover should error")
	}
}
