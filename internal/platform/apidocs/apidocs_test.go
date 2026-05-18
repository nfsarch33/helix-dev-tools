package apidocs

import "testing"

func TestRegister_LookupRoundtrip(t *testing.T) {
	s := NewSpec("Test API", "v1")
	s.Register(Endpoint{Method: "GET", Path: "/products", Summary: "List products"})
	e, ok := s.Lookup("GET", "/products")
	if !ok {
		t.Fatal("expected to find registered endpoint")
	}
	if e.Summary != "List products" {
		t.Errorf("expected 'List products', got %q", e.Summary)
	}
}

func TestLookup_NotFound(t *testing.T) {
	s := NewSpec("Test API", "v1")
	_, ok := s.Lookup("GET", "/missing")
	if ok {
		t.Error("expected false for unknown endpoint")
	}
}

func TestAllEndpoints_ReturnsCopy(t *testing.T) {
	s := NewSpec("Test API", "v1")
	s.Register(Endpoint{Method: "GET", Path: "/a"})
	s.Register(Endpoint{Method: "POST", Path: "/b"})
	all := s.AllEndpoints()
	if len(all) != 2 {
		t.Errorf("expected 2 endpoints, got %d", len(all))
	}
}

func TestValidate_Valid(t *testing.T) {
	s := NewSpec("Test API", "v1")
	s.Register(Endpoint{Method: "GET", Path: "/products"})
	errs := s.Validate()
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidate_MissingMethod(t *testing.T) {
	s := NewSpec("Test API", "v1")
	s.Register(Endpoint{Method: "", Path: "/products"})
	errs := s.Validate()
	if len(errs) == 0 {
		t.Error("expected validation error for missing method")
	}
}

func TestValidate_MissingPath(t *testing.T) {
	s := NewSpec("Test API", "v1")
	s.Register(Endpoint{Method: "GET", Path: ""})
	errs := s.Validate()
	if len(errs) == 0 {
		t.Error("expected validation error for missing path")
	}
}

func TestEndpoint_WithParams(t *testing.T) {
	s := NewSpec("Test API", "v1")
	s.Register(Endpoint{
		Method:  "GET",
		Path:    "/products/{id}",
		Summary: "Get product",
		Params: []Param{
			{Name: "id", In: ParamInPath, Required: true, Type: "string"},
			{Name: "fields", In: ParamInQuery, Required: false, Type: "string"},
		},
	})
	e, _ := s.Lookup("GET", "/products/{id}")
	if len(e.Params) != 2 {
		t.Errorf("expected 2 params, got %d", len(e.Params))
	}
}
