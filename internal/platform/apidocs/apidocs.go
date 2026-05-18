package apidocs

import "fmt"

// ParamIn describes where an API parameter appears
type ParamIn string

const (
	ParamInPath   ParamIn = "path"
	ParamInQuery  ParamIn = "query"
	ParamInHeader ParamIn = "header"
	ParamInBody   ParamIn = "body"
)

// Param describes one API parameter
type Param struct {
	Name     string
	In       ParamIn
	Required bool
	Type     string
}

// Endpoint documents one API route
type Endpoint struct {
	Method  string
	Path    string
	Summary string
	Params  []Param
}

// Spec holds all documented endpoints
type Spec struct {
	Title   string
	Version string
	routes  []Endpoint
}

// NewSpec creates an empty API spec
func NewSpec(title, version string) *Spec {
	return &Spec{Title: title, Version: version}
}

// Register adds an endpoint to the spec
func (s *Spec) Register(e Endpoint) {
	s.routes = append(s.routes, e)
}

// Lookup returns the endpoint matching method+path, or false
func (s *Spec) Lookup(method, path string) (Endpoint, bool) {
	for _, e := range s.routes {
		if e.Method == method && e.Path == path {
			return e, true
		}
	}
	return Endpoint{}, false
}

// AllEndpoints returns a copy of all registered endpoints
func (s *Spec) AllEndpoints() []Endpoint {
	result := make([]Endpoint, len(s.routes))
	copy(result, s.routes)
	return result
}

// Validate returns errors for any endpoint missing Method or Path
func (s *Spec) Validate() []error {
	var errs []error
	for _, e := range s.routes {
		if e.Method == "" {
			errs = append(errs, fmt.Errorf("endpoint %q missing method", e.Path))
		}
		if e.Path == "" {
			errs = append(errs, fmt.Errorf("endpoint %q missing path", e.Method))
		}
	}
	return errs
}
