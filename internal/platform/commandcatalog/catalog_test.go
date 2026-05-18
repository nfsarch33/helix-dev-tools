package commandcatalog

import (
	"testing"
)

func TestRegisterAndGet(t *testing.T) {
	c := New()

	cmd := Command{
		Name:        "workspace-doctor",
		Description: "Run workspace health checks",
		Category:    CatDiagnostic,
		Binary:      "cursor-tools",
		Args:        []string{"workspace", "doctor", "--quick"},
		RequiresSSH: false,
	}

	err := c.Register(cmd)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	got, err := c.Get("workspace-doctor")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Description != "Run workspace health checks" {
		t.Errorf("got desc %q", got.Description)
	}
	if got.Category != CatDiagnostic {
		t.Errorf("got category %q", got.Category)
	}
}

func TestRegisterDuplicate(t *testing.T) {
	c := New()
	c.Register(Command{Name: "x", Category: CatDiagnostic})
	err := c.Register(Command{Name: "x", Category: CatDiagnostic})
	if err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestGetNotFound(t *testing.T) {
	c := New()
	_, err := c.Get("missing")
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestListByCategory(t *testing.T) {
	c := New()
	c.Register(Command{Name: "doctor", Category: CatDiagnostic})
	c.Register(Command{Name: "push", Category: CatGit})
	c.Register(Command{Name: "status", Category: CatGit})
	c.Register(Command{Name: "deploy", Category: CatInfra})

	git := c.ListByCategory(CatGit)
	if len(git) != 2 {
		t.Errorf("expected 2 git commands, got %d", len(git))
	}

	diag := c.ListByCategory(CatDiagnostic)
	if len(diag) != 1 {
		t.Errorf("expected 1 diagnostic, got %d", len(diag))
	}
}

func TestSearch(t *testing.T) {
	c := New()
	c.Register(Command{Name: "workspace-doctor", Description: "Health check", Category: CatDiagnostic, Tags: []string{"health"}})
	c.Register(Command{Name: "fleet-health", Description: "Fleet node check", Category: CatInfra, Tags: []string{"fleet", "health"}})
	c.Register(Command{Name: "git-push", Description: "Push changes", Category: CatGit})

	results := c.Search("health")
	if len(results) != 2 {
		t.Errorf("expected 2 matches for 'health', got %d", len(results))
	}

	results = c.Search("fleet")
	if len(results) != 1 {
		t.Errorf("expected 1 match for 'fleet', got %d", len(results))
	}

	results = c.Search("nonexistent")
	if len(results) != 0 {
		t.Errorf("expected 0 matches, got %d", len(results))
	}
}

func TestAll(t *testing.T) {
	c := New()
	c.Register(Command{Name: "a", Category: CatDiagnostic})
	c.Register(Command{Name: "b", Category: CatGit})

	all := c.All()
	if len(all) != 2 {
		t.Errorf("expected 2, got %d", len(all))
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cmd     Command
		wantErr bool
	}{
		{"valid", Command{Name: "x", Description: "desc", Category: CatDiagnostic}, false},
		{"empty name", Command{Name: "", Description: "x", Category: CatGit}, true},
		{"empty desc", Command{Name: "x", Description: "", Category: CatGit}, true},
		{"invalid cat", Command{Name: "x", Description: "x", Category: "nope"}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.cmd)
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() err=%v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

func TestListByAgent(t *testing.T) {
	c := New()
	c.Register(Command{Name: "a", Category: CatDiagnostic, AllowedAgents: []string{"cursor-parent", "codex"}})
	c.Register(Command{Name: "b", Category: CatGit, AllowedAgents: []string{"cursor-parent"}})
	c.Register(Command{Name: "c", Category: CatInfra, AllowedAgents: []string{"codex"}})
	c.Register(Command{Name: "d", Category: CatDiagnostic}) // no restriction = all agents

	cursor := c.ListByAgent("cursor-parent")
	if len(cursor) != 3 {
		t.Errorf("expected 3 for cursor-parent, got %d", len(cursor))
	}

	codex := c.ListByAgent("codex")
	if len(codex) != 3 {
		t.Errorf("expected 3 for codex, got %d", len(codex))
	}
}

func TestDangerousFlag(t *testing.T) {
	c := New()
	c.Register(Command{Name: "safe", Category: CatGit, Dangerous: false})
	c.Register(Command{Name: "force-push", Category: CatGit, Dangerous: true})
	c.Register(Command{Name: "reset-hard", Category: CatGit, Dangerous: true})

	dangerous := c.ListDangerous()
	if len(dangerous) != 2 {
		t.Errorf("expected 2 dangerous, got %d", len(dangerous))
	}
}
