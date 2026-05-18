package componentregistry

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

type Category string

const (
	CategoryService   Category = "service"
	CategoryDaemon    Category = "daemon"
	CategoryInfra     Category = "infra"
	CategoryValidator Category = "validator"
	CategoryMCP       Category = "mcp"
	CategoryHook      Category = "hook"
	CategoryRule      Category = "rule"
	CategorySkill     Category = "skill"
	CategoryCLI       Category = "cli"
)

type Status string

const (
	StatusUnknown  Status = "unknown"
	StatusHealthy  Status = "healthy"
	StatusDegraded Status = "degraded"
	StatusDown     Status = "down"
)

type Component struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Category    Category `json:"category"`
	InstallPath string   `json:"install_path,omitempty"`
	ConfigPath  string   `json:"config_path,omitempty"`
	HealthCheck string   `json:"health_check,omitempty"`
	Owner       string   `json:"owner,omitempty"`
	Node        string   `json:"node,omitempty"`
	Status      Status   `json:"status"`
	LastChecked time.Time `json:"last_checked,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

type Snapshot struct {
	Total     int       `json:"total"`
	Healthy   int       `json:"healthy"`
	Degraded  int       `json:"degraded"`
	Down      int       `json:"down"`
	Unknown   int       `json:"unknown"`
	Timestamp time.Time `json:"timestamp"`
}

var validCategories = map[Category]bool{
	CategoryService:   true,
	CategoryDaemon:    true,
	CategoryInfra:     true,
	CategoryValidator: true,
	CategoryMCP:       true,
	CategoryHook:      true,
	CategoryRule:      true,
	CategorySkill:     true,
	CategoryCLI:       true,
}

type Registry struct {
	mu         sync.RWMutex
	components map[string]Component
}

func New() *Registry {
	return &Registry{
		components: make(map[string]Component),
	}
}

func Validate(c Component) error {
	if c.ID == "" {
		return errors.New("component ID is required")
	}
	if c.Name == "" {
		return errors.New("component Name is required")
	}
	if !validCategories[c.Category] {
		return fmt.Errorf("invalid category %q", c.Category)
	}
	return nil
}

func (r *Registry) Register(c Component) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.components[c.ID]; exists {
		return fmt.Errorf("component %q already registered", c.ID)
	}

	if c.Status == "" {
		c.Status = StatusUnknown
	}

	r.components[c.ID] = c
	return nil
}

func (r *Registry) Deregister(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.components[id]; !exists {
		return fmt.Errorf("component %q not found", id)
	}

	delete(r.components, id)
	return nil
}

func (r *Registry) Lookup(id string) (Component, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	c, exists := r.components[id]
	if !exists {
		return Component{}, fmt.Errorf("component %q not found", id)
	}
	return c, nil
}

func (r *Registry) UpdateStatus(id string, status Status) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, exists := r.components[id]
	if !exists {
		return fmt.Errorf("component %q not found", id)
	}

	c.Status = status
	c.LastChecked = time.Now()
	r.components[id] = c
	return nil
}

func (r *Registry) ListByCategory(cat Category) []Component {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []Component
	for _, c := range r.components {
		if c.Category == cat {
			result = append(result, c)
		}
	}
	return result
}

func (r *Registry) ListByNode(node string) []Component {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []Component
	for _, c := range r.components {
		if c.Node == node {
			result = append(result, c)
		}
	}
	return result
}

func (r *Registry) All() []Component {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Component, 0, len(r.components))
	for _, c := range r.components {
		result = append(result, c)
	}
	return result
}

func (r *Registry) Snapshot() Snapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()

	snap := Snapshot{
		Total:     len(r.components),
		Timestamp: time.Now(),
	}

	for _, c := range r.components {
		switch c.Status {
		case StatusHealthy:
			snap.Healthy++
		case StatusDegraded:
			snap.Degraded++
		case StatusDown:
			snap.Down++
		default:
			snap.Unknown++
		}
	}
	return snap
}
