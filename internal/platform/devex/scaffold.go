package devex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Template struct {
	Name  string
	Files map[string]string
}

type Scaffolder struct {
	templates map[string]Template
}

func NewScaffolder() *Scaffolder {
	return &Scaffolder{templates: make(map[string]Template)}
}

func (s *Scaffolder) Register(tmpl Template) {
	s.templates[tmpl.Name] = tmpl
}

func (s *Scaffolder) Generate(templateName, targetDir string, vars map[string]string) error {
	tmpl, ok := s.templates[templateName]
	if !ok {
		return fmt.Errorf("template %q not found", templateName)
	}

	for relPath, content := range tmpl.Files {
		rendered := applyVars(content, vars)
		renderedPath := applyVars(relPath, vars)
		fullPath := filepath.Join(targetDir, renderedPath)

		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create dir %s: %w", dir, err)
		}
		if err := os.WriteFile(fullPath, []byte(rendered), 0644); err != nil {
			return fmt.Errorf("write %s: %w", fullPath, err)
		}
	}
	return nil
}

func (s *Scaffolder) List() []string {
	names := make([]string, 0, len(s.templates))
	for name := range s.templates {
		names = append(names, name)
	}
	return names
}

func (s *Scaffolder) FileCount(templateName string) int {
	tmpl, ok := s.templates[templateName]
	if !ok {
		return 0
	}
	return len(tmpl.Files)
}

func applyVars(content string, vars map[string]string) string {
	result := content
	for k, v := range vars {
		result = strings.ReplaceAll(result, "{{"+k+"}}", v)
	}
	return result
}
