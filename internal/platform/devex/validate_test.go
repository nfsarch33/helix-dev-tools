package devex

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateJSON_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte(`{"name": "test", "port": 8080}`), 0644)

	data, errs := ValidateJSON(path)
	if len(errs) > 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
	if data["name"] != "test" {
		t.Error("expected name=test")
	}
}

func TestValidateJSON_Invalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte(`{invalid`), 0644)

	_, errs := ValidateJSON(path)
	if len(errs) != 1 {
		t.Errorf("expected 1 error, got %d", len(errs))
	}
}

func TestValidateJSON_Missing(t *testing.T) {
	_, errs := ValidateJSON("/nonexistent/file.json")
	if len(errs) != 1 {
		t.Error("expected error for missing file")
	}
}

func TestValidateSchema_RequiredMissing(t *testing.T) {
	schema := Schema{Fields: []SchemaField{
		{Name: "host", Type: "string", Required: true},
		{Name: "port", Type: "number", Required: true},
	}}

	errs := ValidateSchema(map[string]interface{}{"host": "localhost"}, schema)
	if len(errs) != 1 {
		t.Errorf("expected 1 error for missing port, got %d", len(errs))
	}
}

func TestValidateSchema_TypeMismatch(t *testing.T) {
	schema := Schema{Fields: []SchemaField{
		{Name: "port", Type: "number", Required: true},
	}}

	errs := ValidateSchema(map[string]interface{}{"port": "not-a-number"}, schema)
	if len(errs) != 1 {
		t.Errorf("expected type mismatch error")
	}
}

func TestValidateSchema_AllValid(t *testing.T) {
	schema := Schema{Fields: []SchemaField{
		{Name: "name", Type: "string", Required: true},
		{Name: "count", Type: "number", Required: false},
	}}

	errs := ValidateSchema(map[string]interface{}{"name": "test", "count": float64(5)}, schema)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}
