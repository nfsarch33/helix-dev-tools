package intelligence

import "testing"

func TestRouter_RegisterAndRoute(t *testing.T) {
	r := NewRouter()
	r.Register(ModelConfig{Name: "fast-model", Tier: TierFast, CostPer1K: 0.001})

	m, err := r.Route(TierFast)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Name != "fast-model" {
		t.Errorf("expected fast-model, got %s", m.Name)
	}
}

func TestRouter_RouteEmpty(t *testing.T) {
	r := NewRouter()
	_, err := r.Route(TierQuality)
	if err == nil {
		t.Error("expected error for empty tier")
	}
}

func TestRouter_RouteByBudget(t *testing.T) {
	r := NewRouter()
	r.Register(ModelConfig{Name: "cheap", Tier: TierFast, CostPer1K: 0.001})
	r.Register(ModelConfig{Name: "expensive", Tier: TierQuality, CostPer1K: 0.1})

	m, err := r.RouteByBudget(0.01)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Name != "cheap" {
		t.Errorf("expected cheap model, got %s", m.Name)
	}
}

func TestRouter_RouteByBudget_NoneAvailable(t *testing.T) {
	r := NewRouter()
	r.Register(ModelConfig{Name: "pricey", Tier: TierQuality, CostPer1K: 1.0})

	_, err := r.RouteByBudget(0.001)
	if err == nil {
		t.Error("expected error when no model within budget")
	}
}

func TestRouter_ModelCount(t *testing.T) {
	r := NewRouter()
	r.Register(ModelConfig{Name: "a", Tier: TierFast})
	r.Register(ModelConfig{Name: "b", Tier: TierBalanced})

	if r.ModelCount() != 2 {
		t.Errorf("expected 2, got %d", r.ModelCount())
	}
}

func TestRouter_AllModels(t *testing.T) {
	r := NewRouter()
	r.Register(ModelConfig{Name: "a", Tier: TierFast})
	r.Register(ModelConfig{Name: "b", Tier: TierQuality})

	all := r.AllModels()
	if len(all) != 2 {
		t.Errorf("expected 2, got %d", len(all))
	}
}
