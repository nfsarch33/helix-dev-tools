package freshinstall

import "testing"

func TestSuite_AllPass(t *testing.T) {
	vs := NewValidationSuite()
	vs.Register(CategoryMCP, "mem0-reachable", func() (bool, string) { return true, "ok" })
	vs.Register(CategoryHooks, "post-edit-fires", func() (bool, string) { return true, "ok" })
	results := vs.Run()
	p, f := Summary(results)
	if p != 2 || f != 0 {
		t.Errorf("expected 2 passed 0 failed, got %d/%d", p, f)
	}
}

func TestSuite_OneFails(t *testing.T) {
	vs := NewValidationSuite()
	vs.Register(CategoryTools, "cursor-tools", func() (bool, string) { return true, "v9.0.0" })
	vs.Register(CategoryTools, "sentrux", func() (bool, string) { return false, "not found" })
	results := vs.Run()
	p, f := Summary(results)
	if p != 1 || f != 1 {
		t.Errorf("expected 1 passed 1 failed, got %d/%d", p, f)
	}
}

func TestByCategory_FiltersCorrectly(t *testing.T) {
	vs := NewValidationSuite()
	vs.Register(CategoryMCP, "mcp1", func() (bool, string) { return true, "" })
	vs.Register(CategoryMCP, "mcp2", func() (bool, string) { return true, "" })
	vs.Register(CategoryHooks, "hook1", func() (bool, string) { return true, "" })
	results := vs.Run()
	mcpResults := ByCategory(results, CategoryMCP)
	if len(mcpResults) != 2 {
		t.Errorf("expected 2 MCP results, got %d", len(mcpResults))
	}
}

func TestSummary_Empty(t *testing.T) {
	p, f := Summary(nil)
	if p != 0 || f != 0 {
		t.Error("expected 0/0 for empty results")
	}
}
