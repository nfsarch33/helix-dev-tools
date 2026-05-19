package intelligence

import "fmt"

type ModelTier int

const (
	TierFast ModelTier = iota
	TierBalanced
	TierQuality
)

type ModelConfig struct {
	Name      string
	Tier      ModelTier
	CostPer1K float64
	MaxTokens int
	Endpoint  string
}

type Router struct {
	models map[ModelTier][]ModelConfig
}

func NewRouter() *Router {
	return &Router{models: make(map[ModelTier][]ModelConfig)}
}

func (r *Router) Register(cfg ModelConfig) {
	r.models[cfg.Tier] = append(r.models[cfg.Tier], cfg)
}

func (r *Router) Route(tier ModelTier) (ModelConfig, error) {
	models := r.models[tier]
	if len(models) == 0 {
		return ModelConfig{}, fmt.Errorf("no models registered for tier %d", tier)
	}
	return models[0], nil
}

func (r *Router) RouteByBudget(maxCostPer1K float64) (ModelConfig, error) {
	for tier := TierFast; tier <= TierQuality; tier++ {
		for _, m := range r.models[tier] {
			if m.CostPer1K <= maxCostPer1K {
				return m, nil
			}
		}
	}
	return ModelConfig{}, fmt.Errorf("no model within budget $%.4f/1K", maxCostPer1K)
}

func (r *Router) AllModels() []ModelConfig {
	var all []ModelConfig
	for tier := TierFast; tier <= TierQuality; tier++ {
		all = append(all, r.models[tier]...)
	}
	return all
}

func (r *Router) ModelCount() int {
	count := 0
	for _, models := range r.models {
		count += len(models)
	}
	return count
}
