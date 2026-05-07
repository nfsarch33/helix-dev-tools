package mem0

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

var safeNamespacePart = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

// CRMNamespaceConfig defines one isolated CRM memory namespace.
type CRMNamespaceConfig struct {
	TenantID        string
	CRMProvider     string
	ContextID       string
	RetentionDays   int
	BaseAppID       string
	AllowedContexts []string
}

// SearchQuery is the metadata envelope required for CRM memory reads.
type SearchQuery struct {
	TenantID    string
	CRMProvider string
	ContextID   string
	Text        string
}

// CRMNamespace enforces tenant and context isolation before Mem0 search.
type CRMNamespace struct {
	cfg CRMNamespaceConfig
}

// NewCRMNamespace validates and creates one CRM namespace.
func NewCRMNamespace(cfg CRMNamespaceConfig) (CRMNamespace, error) {
	if cfg.BaseAppID == "" {
		cfg.BaseAppID = "cursor-global-kb"
	}
	if cfg.RetentionDays <= 0 {
		cfg.RetentionDays = 30
	}
	for label, value := range map[string]string{
		"tenant_id":    cfg.TenantID,
		"crm_provider": strings.ToLower(cfg.CRMProvider),
		"context_id":   cfg.ContextID,
		"base_app_id":  cfg.BaseAppID,
	} {
		if !safeNamespacePart.MatchString(value) {
			return CRMNamespace{}, fmt.Errorf("crm namespace: unsafe %s %q", label, value)
		}
	}
	cfg.CRMProvider = strings.ToLower(cfg.CRMProvider)
	return CRMNamespace{cfg: cfg}, nil
}

// AppID returns the Mem0 app_id for this isolated CRM context.
func (n CRMNamespace) AppID() string {
	return fmt.Sprintf("%s:crm:%s:%s:%s", n.cfg.BaseAppID, n.cfg.CRMProvider, n.cfg.TenantID, n.cfg.ContextID)
}

// RetentionDays returns the configured retention policy.
func (n CRMNamespace) RetentionDays() int {
	return n.cfg.RetentionDays
}

// Authorize verifies the query stays inside the configured CRM boundary.
func (n CRMNamespace) Authorize(q SearchQuery) error {
	if q.TenantID != n.cfg.TenantID {
		return fmt.Errorf("crm namespace: tenant mismatch")
	}
	if strings.ToLower(q.CRMProvider) != n.cfg.CRMProvider {
		return fmt.Errorf("crm namespace: provider mismatch")
	}
	if q.ContextID != n.cfg.ContextID {
		return fmt.Errorf("crm namespace: context mismatch")
	}
	if len(n.cfg.AllowedContexts) > 0 && !slices.Contains(n.cfg.AllowedContexts, q.ContextID) {
		return fmt.Errorf("crm namespace: context not allowed")
	}
	return nil
}
