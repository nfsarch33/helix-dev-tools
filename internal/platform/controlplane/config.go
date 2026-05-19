package controlplane

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type ConfigValue struct {
	Key       string
	Value     interface{}
	UpdatedAt time.Time
}

type ConfigStore struct {
	mu     sync.RWMutex
	values map[string]ConfigValue
	path   string
}

func NewConfigStore(path string) *ConfigStore {
	return &ConfigStore{
		values: make(map[string]ConfigValue),
		path:   path,
	}
}

func (c *ConfigStore) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values[key] = ConfigValue{Key: key, Value: value, UpdatedAt: time.Now()}
}

func (c *ConfigStore) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.values[key]
	if !ok {
		return nil, false
	}
	return v.Value, true
}

func (c *ConfigStore) GetString(key, defaultVal string) string {
	v, ok := c.Get(key)
	if !ok {
		return defaultVal
	}
	s, ok := v.(string)
	if !ok {
		return defaultVal
	}
	return s
}

func (c *ConfigStore) All() map[string]ConfigValue {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make(map[string]ConfigValue, len(c.values))
	for k, v := range c.values {
		result[k] = v
	}
	return result
}

func (c *ConfigStore) LoadFromFile() error {
	if c.path == "" {
		return fmt.Errorf("no config path set")
	}
	data, err := os.ReadFile(c.path)
	if err != nil {
		return err
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, v := range raw {
		c.values[k] = ConfigValue{Key: k, Value: v, UpdatedAt: time.Now()}
	}
	return nil
}

func (c *ConfigStore) Validate(required []string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var missing []string
	for _, key := range required {
		if _, ok := c.values[key]; !ok {
			missing = append(missing, key)
		}
	}
	return missing
}
