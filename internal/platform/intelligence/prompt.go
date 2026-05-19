package intelligence

import (
	"fmt"
	"strings"
)

type PromptTemplate struct {
	Name     string
	Template string
	Vars     []string
}

type PromptEngine struct {
	templates map[string]PromptTemplate
}

func NewPromptEngine() *PromptEngine {
	return &PromptEngine{templates: make(map[string]PromptTemplate)}
}

func (e *PromptEngine) Register(tmpl PromptTemplate) {
	e.templates[tmpl.Name] = tmpl
}

func (e *PromptEngine) Render(name string, vars map[string]string) (string, error) {
	tmpl, ok := e.templates[name]
	if !ok {
		return "", fmt.Errorf("template %q not found", name)
	}

	result := tmpl.Template
	for _, v := range tmpl.Vars {
		placeholder := "{{" + v + "}}"
		val, exists := vars[v]
		if !exists {
			return "", fmt.Errorf("missing variable %q for template %q", v, name)
		}
		result = strings.ReplaceAll(result, placeholder, val)
	}
	return result, nil
}

func (e *PromptEngine) List() []string {
	names := make([]string, 0, len(e.templates))
	for name := range e.templates {
		names = append(names, name)
	}
	return names
}

func (e *PromptEngine) RequiredVars(name string) ([]string, error) {
	tmpl, ok := e.templates[name]
	if !ok {
		return nil, fmt.Errorf("template %q not found", name)
	}
	return tmpl.Vars, nil
}
