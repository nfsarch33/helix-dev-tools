package commandcatalog

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

type CommandCategory string

const (
	CatDiagnostic CommandCategory = "diagnostic"
	CatGit        CommandCategory = "git"
	CatInfra      CommandCategory = "infra"
	CatMemory     CommandCategory = "memory"
	CatSprint     CommandCategory = "sprint"
	CatBuild      CommandCategory = "build"
	CatDeploy     CommandCategory = "deploy"
	CatMonitor    CommandCategory = "monitor"
)

var validCategories = map[CommandCategory]bool{
	CatDiagnostic: true,
	CatGit:        true,
	CatInfra:      true,
	CatMemory:     true,
	CatSprint:     true,
	CatBuild:      true,
	CatDeploy:     true,
	CatMonitor:    true,
}

type Command struct {
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	Category      CommandCategory `json:"category"`
	Binary        string          `json:"binary,omitempty"`
	Args          []string        `json:"args,omitempty"`
	RequiresSSH   bool            `json:"requires_ssh,omitempty"`
	Dangerous     bool            `json:"dangerous,omitempty"`
	AllowedAgents []string        `json:"allowed_agents,omitempty"`
	Tags          []string        `json:"tags,omitempty"`
}

type Catalog struct {
	mu       sync.RWMutex
	commands map[string]Command
}

func New() *Catalog {
	return &Catalog{
		commands: make(map[string]Command),
	}
}

func Validate(cmd Command) error {
	if cmd.Name == "" {
		return errors.New("command name is required")
	}
	if cmd.Description == "" {
		return errors.New("command description is required")
	}
	if !validCategories[cmd.Category] {
		return fmt.Errorf("invalid category %q", cmd.Category)
	}
	return nil
}

func (c *Catalog) Register(cmd Command) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.commands[cmd.Name]; exists {
		return fmt.Errorf("command %q already registered", cmd.Name)
	}

	c.commands[cmd.Name] = cmd
	return nil
}

func (c *Catalog) Get(name string) (Command, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cmd, exists := c.commands[name]
	if !exists {
		return Command{}, fmt.Errorf("command %q not found", name)
	}
	return cmd, nil
}

func (c *Catalog) ListByCategory(cat CommandCategory) []Command {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []Command
	for _, cmd := range c.commands {
		if cmd.Category == cat {
			result = append(result, cmd)
		}
	}
	return result
}

func (c *Catalog) ListByAgent(agent string) []Command {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []Command
	for _, cmd := range c.commands {
		if len(cmd.AllowedAgents) == 0 {
			result = append(result, cmd)
			continue
		}
		for _, a := range cmd.AllowedAgents {
			if a == agent {
				result = append(result, cmd)
				break
			}
		}
	}
	return result
}

func (c *Catalog) ListDangerous() []Command {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []Command
	for _, cmd := range c.commands {
		if cmd.Dangerous {
			result = append(result, cmd)
		}
	}
	return result
}

func (c *Catalog) Search(query string) []Command {
	c.mu.RLock()
	defer c.mu.RUnlock()

	q := strings.ToLower(query)
	var result []Command
	for _, cmd := range c.commands {
		if matchesQuery(cmd, q) {
			result = append(result, cmd)
		}
	}
	return result
}

func matchesQuery(cmd Command, q string) bool {
	if strings.Contains(strings.ToLower(cmd.Name), q) {
		return true
	}
	if strings.Contains(strings.ToLower(cmd.Description), q) {
		return true
	}
	for _, tag := range cmd.Tags {
		if strings.Contains(strings.ToLower(tag), q) {
			return true
		}
	}
	return false
}

func (c *Catalog) All() []Command {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]Command, 0, len(c.commands))
	for _, cmd := range c.commands {
		result = append(result, cmd)
	}
	return result
}
