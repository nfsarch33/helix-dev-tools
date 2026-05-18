package hookmigrate

import "fmt"

// HookType identifies which lifecycle hook is being migrated
type HookType string

const (
	HookPreEdit  HookType = "pre-edit"
	HookPostEdit HookType = "post-edit"
	HookPreShell HookType = "pre-shell"
	HookAgentStart HookType = "agent-start"
	HookAgentStop  HookType = "agent-stop"
)

// MigrationStatus tracks whether a hook has been migrated from shell to Go
type MigrationStatus string

const (
	StatusPending   MigrationStatus = "pending"
	StatusMigrated  MigrationStatus = "migrated"
	StatusVerified  MigrationStatus = "verified"
)

// HookRecord holds migration state for one hook
type HookRecord struct {
	Name       string
	Type       HookType
	ShellPath  string
	GoSubcmd   string
	Status     MigrationStatus
}

// Registry tracks all hook migrations
type Registry struct {
	hooks []HookRecord
}

// NewRegistry creates an empty registry
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds a hook to track
func (r *Registry) Register(h HookRecord) {
	r.hooks = append(r.hooks, h)
}

// Migrate marks a hook as migrated to Go
func (r *Registry) Migrate(name string) error {
	for i := range r.hooks {
		if r.hooks[i].Name == name {
			if r.hooks[i].Status != StatusPending {
				return fmt.Errorf("hook %q is not in pending state", name)
			}
			r.hooks[i].Status = StatusMigrated
			return nil
		}
	}
	return fmt.Errorf("hook %q not found", name)
}

// Verify marks a hook as verified (tested in Go)
func (r *Registry) Verify(name string) error {
	for i := range r.hooks {
		if r.hooks[i].Name == name {
			if r.hooks[i].Status != StatusMigrated {
				return fmt.Errorf("hook %q must be migrated before verification", name)
			}
			r.hooks[i].Status = StatusVerified
			return nil
		}
	}
	return fmt.Errorf("hook %q not found", name)
}

// PendingCount returns the number of hooks still pending migration
func (r *Registry) PendingCount() int {
	n := 0
	for _, h := range r.hooks {
		if h.Status == StatusPending {
			n++
		}
	}
	return n
}

// All returns a copy of all hook records
func (r *Registry) All() []HookRecord {
	result := make([]HookRecord, len(r.hooks))
	copy(result, r.hooks)
	return result
}
