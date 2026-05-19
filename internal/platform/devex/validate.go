package devex

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type ValidationError struct {
	Path    string
	Message string
}

type SchemaField struct {
	Name     string
	Type     string
	Required bool
}

type Schema struct {
	Name   string
	Fields []SchemaField
}

func ValidateJSON(path string) (map[string]interface{}, []ValidationError) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, []ValidationError{{Path: path, Message: fmt.Sprintf("read error: %v", err)}}
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, []ValidationError{{Path: path, Message: fmt.Sprintf("invalid JSON: %v", err)}}
	}
	return result, nil
}

func ValidateSchema(data map[string]interface{}, schema Schema) []ValidationError {
	var errs []ValidationError
	for _, field := range schema.Fields {
		val, exists := data[field.Name]
		if !exists {
			if field.Required {
				errs = append(errs, ValidationError{Path: field.Name, Message: "required field missing"})
			}
			continue
		}
		if !matchesType(val, field.Type) {
			errs = append(errs, ValidationError{
				Path:    field.Name,
				Message: fmt.Sprintf("expected type %s, got %T", field.Type, val),
			})
		}
	}
	return errs
}

func matchesType(val interface{}, expectedType string) bool {
	switch strings.ToLower(expectedType) {
	case "string":
		_, ok := val.(string)
		return ok
	case "number":
		_, ok := val.(float64)
		return ok
	case "bool", "boolean":
		_, ok := val.(bool)
		return ok
	case "array":
		_, ok := val.([]interface{})
		return ok
	case "object":
		_, ok := val.(map[string]interface{})
		return ok
	default:
		return true
	}
}
