package routeaudit

import "testing"

func TestAudit_AllLocal(t *testing.T) {
	routes := []Route{
		{Name: "vllm-main", Backend: "local-vllm", Active: true},
		{Name: "vllm-backup", Backend: "local-vllm", Active: true},
	}
	res := Audit(routes)
	if !res.LocalOnlyOK {
		t.Error("expected LocalOnlyOK=true")
	}
	if res.ExternalRoutes != 0 {
		t.Errorf("expected 0 external, got %d", res.ExternalRoutes)
	}
}

func TestAudit_MiniMaxRetired(t *testing.T) {
	routes := []Route{
		{Name: "minimax-bridge", Backend: "minimax", Active: false},
		{Name: "vllm-main", Backend: "local-vllm", Active: true},
	}
	res := Audit(routes)
	if len(res.RetiredRoutes) != 1 || res.RetiredRoutes[0] != "minimax-bridge" {
		t.Errorf("expected minimax-bridge in retired, got %v", res.RetiredRoutes)
	}
	if !res.LocalOnlyOK {
		t.Error("expected LocalOnlyOK after MiniMax retired")
	}
}

func TestAudit_ExternalStillActive(t *testing.T) {
	routes := []Route{
		{Name: "openai-fallback", Backend: "openai", Active: true},
		{Name: "vllm-main", Backend: "local-vllm", Active: true},
	}
	res := Audit(routes)
	if res.LocalOnlyOK {
		t.Error("expected LocalOnlyOK=false when external route active")
	}
	if res.ExternalRoutes != 1 {
		t.Errorf("expected 1 external, got %d", res.ExternalRoutes)
	}
}

func TestAudit_EmptyRoutes(t *testing.T) {
	res := Audit(nil)
	if res.TotalRoutes != 0 {
		t.Errorf("expected 0 total routes, got %d", res.TotalRoutes)
	}
	if res.LocalOnlyOK {
		t.Error("expected LocalOnlyOK=false for empty routes")
	}
}

func TestRetiredNames(t *testing.T) {
	routes := []Route{
		{Name: "minimax", Active: false},
		{Name: "vllm", Active: true},
	}
	names := RetiredNames(routes)
	if len(names) != 1 || names[0] != "minimax" {
		t.Errorf("expected [minimax], got %v", names)
	}
}
