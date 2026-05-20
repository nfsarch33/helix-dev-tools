package cli

import (
	"testing"
)

func TestEvaluateSembleDiscipline_ExploratoryAsk(t *testing.T) {
	t.Setenv("CURSOR_TOOLS_SEMBLE_STRICT", "")
	resp := evaluateSembleDiscipline(`rg "foo" .`, "test")
	if resp.Permission != "ask" {
		t.Fatalf("got permission %q, want ask", resp.Permission)
	}
}

func TestEvaluateSembleDiscipline_StrictDeny(t *testing.T) {
	t.Setenv("CURSOR_TOOLS_SEMBLE_STRICT", "1")
	resp := evaluateSembleDiscipline(`rg "foo" .`, "test")
	if resp.Permission != "deny" {
		t.Fatalf("got permission %q, want deny", resp.Permission)
	}
}

func TestEvaluateSembleDiscipline_LiteralAllow(t *testing.T) {
	resp := evaluateSembleDiscipline(`rg -F 'ErrX' .`, "test")
	if resp.Permission != "allow" {
		t.Fatalf("got permission %q, want allow", resp.Permission)
	}
}

func TestEvaluateSembleDiscipline_EmptyAllow(t *testing.T) {
	resp := evaluateSembleDiscipline("", "test")
	if resp == nil || resp.Permission != "allow" {
		t.Fatalf("got %+v", resp)
	}
}
