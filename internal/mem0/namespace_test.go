package mem0

import "testing"

func TestNamespace_PerCRMContextIsolation(t *testing.T) {
	t.Parallel()
	ns, err := NewCRMNamespace(CRMNamespaceConfig{
		TenantID:        "tenant-a",
		CRMProvider:     "hubspot",
		ContextID:       "deal-42",
		RetentionDays:   90,
		BaseAppID:       "cursor-global-kb",
		AllowedContexts: []string{"deal-42"},
	})
	if err != nil {
		t.Fatalf("NewCRMNamespace: %v", err)
	}

	query := SearchQuery{TenantID: "tenant-a", CRMProvider: "hubspot", ContextID: "deal-42", Text: "follow up"}
	if err := ns.Authorize(query); err != nil {
		t.Fatalf("Authorize same context: %v", err)
	}

	crossTenant := SearchQuery{TenantID: "tenant-b", CRMProvider: "hubspot", ContextID: "deal-42", Text: "follow up"}
	if err := ns.Authorize(crossTenant); err == nil {
		t.Fatal("expected cross-tenant query to be rejected")
	}

	crossContext := SearchQuery{TenantID: "tenant-a", CRMProvider: "hubspot", ContextID: "deal-99", Text: "follow up"}
	if err := ns.Authorize(crossContext); err == nil {
		t.Fatal("expected cross-context query to be rejected")
	}

	if ns.AppID() != "cursor-global-kb:crm:hubspot:tenant-a:deal-42" {
		t.Fatalf("app id = %q", ns.AppID())
	}
	if ns.RetentionDays() != 90 {
		t.Fatalf("retention days = %d", ns.RetentionDays())
	}
}

func TestNamespace_RejectsUnsafeContext(t *testing.T) {
	t.Parallel()
	if _, err := NewCRMNamespace(CRMNamespaceConfig{
		TenantID:      "tenant-a",
		CRMProvider:   "hubspot",
		ContextID:     "../deal",
		RetentionDays: 30,
	}); err == nil {
		t.Fatal("expected unsafe context id to be rejected")
	}
}
