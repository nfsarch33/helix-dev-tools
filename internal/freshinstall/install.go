package freshinstall

import "sync"

type Config struct {
	HomePath string
}

type Component struct {
	Name       string
	BinaryPath string
	Required   bool
}

type CheckResult struct {
	Exists  bool
	Version string
	Healthy bool
	Error   string
}

type ChecklistItem struct {
	Component Component
	Required  bool
	Status    string
}

type Validator struct {
	config     Config
	mu         sync.Mutex
	components []Component
	results    map[string]CheckResult
}

func New(cfg Config) *Validator {
	return &Validator{
		config:  cfg,
		results: make(map[string]CheckResult),
	}
}

func (v *Validator) AddComponent(c Component) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.components = append(v.components, c)
}

func (v *Validator) Components() []Component {
	v.mu.Lock()
	defer v.mu.Unlock()
	out := make([]Component, len(v.components))
	copy(out, v.components)
	return out
}

func (v *Validator) RecordCheck(name string, r CheckResult) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.results[name] = r
}

func (v *Validator) Results() map[string]CheckResult {
	v.mu.Lock()
	defer v.mu.Unlock()
	out := make(map[string]CheckResult, len(v.results))
	for k, val := range v.results {
		out[k] = val
	}
	return out
}

func (v *Validator) AllHealthy() bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	for _, r := range v.results {
		if !r.Healthy {
			return false
		}
	}
	return true
}

func (v *Validator) GenerateChecklist() []ChecklistItem {
	v.mu.Lock()
	defer v.mu.Unlock()
	items := make([]ChecklistItem, len(v.components))
	for i, c := range v.components {
		status := "unchecked"
		if r, ok := v.results[c.Name]; ok {
			if r.Healthy {
				status = "pass"
			} else {
				status = "fail"
			}
		}
		items[i] = ChecklistItem{Component: c, Required: c.Required, Status: status}
	}
	return items
}
