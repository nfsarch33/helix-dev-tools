package eval

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func LoadEvalFile(path string) (EvalDef, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return EvalDef{}, errorf("read eval file %s: %w", path, err)
	}

	var def EvalDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		return EvalDef{}, errorf("parse eval YAML %s: %w", path, err)
	}

	if def.ID == "" {
		base := filepath.Base(path)
		def.ID = strings.TrimSuffix(base, filepath.Ext(base))
	}

	return def, def.Validate()
}

func ListEvalFiles(dir string) ([]string, error) {
	var files []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, errorf("read eval dir %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
			files = append(files, filepath.Join(dir, name))
		}
	}
	return files, nil
}

func RunEvalFile(path string) (EvalResult, error) {
	def, err := LoadEvalFile(path)
	if err != nil {
		return EvalResult{}, err
	}
	runner := NewRunner()
	return runner.Run(def), nil
}

func RunAllEvalsInDir(dir string) ([]EvalResult, error) {
	files, err := ListEvalFiles(dir)
	if err != nil {
		return nil, err
	}
	var results []EvalResult
	for _, f := range files {
		result, err := RunEvalFile(f)
		if err != nil {
			results = append(results, EvalResult{
				EvalID: filepath.Base(f),
				Pass:   false,
				Error:  err.Error(),
			})
			continue
		}
		results = append(results, result)
	}
	return results, nil
}
