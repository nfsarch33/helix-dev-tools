package intelligence

import "testing"

func TestPromptEngine_RenderSuccess(t *testing.T) {
	e := NewPromptEngine()
	e.Register(PromptTemplate{
		Name:     "extract",
		Template: "Extract facts from: {{text}}. Format: {{format}}",
		Vars:     []string{"text", "format"},
	})

	result, err := e.Render("extract", map[string]string{"text": "hello world", "format": "json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Extract facts from: hello world. Format: json" {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestPromptEngine_MissingVar(t *testing.T) {
	e := NewPromptEngine()
	e.Register(PromptTemplate{Name: "t", Template: "{{x}}", Vars: []string{"x"}})

	_, err := e.Render("t", map[string]string{})
	if err == nil {
		t.Error("expected error for missing variable")
	}
}

func TestPromptEngine_TemplateNotFound(t *testing.T) {
	e := NewPromptEngine()
	_, err := e.Render("nonexistent", nil)
	if err == nil {
		t.Error("expected error for missing template")
	}
}

func TestPromptEngine_List(t *testing.T) {
	e := NewPromptEngine()
	e.Register(PromptTemplate{Name: "a"})
	e.Register(PromptTemplate{Name: "b"})

	if len(e.List()) != 2 {
		t.Errorf("expected 2 templates, got %d", len(e.List()))
	}
}

func TestPromptEngine_RequiredVars(t *testing.T) {
	e := NewPromptEngine()
	e.Register(PromptTemplate{Name: "t", Vars: []string{"x", "y"}})

	vars, err := e.RequiredVars("t")
	if err != nil {
		t.Fatal(err)
	}
	if len(vars) != 2 {
		t.Errorf("expected 2 vars, got %d", len(vars))
	}
}
