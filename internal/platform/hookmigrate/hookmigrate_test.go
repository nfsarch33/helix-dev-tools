package hookmigrate

import "testing"

func TestRegister_Migrate_Verify(t *testing.T) {
	r := NewRegistry()
	r.Register(HookRecord{Name: "post-edit", Type: HookPostEdit, ShellPath: "post-edit.sh", Status: StatusPending})
	if err := r.Migrate("post-edit"); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if err := r.Verify("post-edit"); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	hooks := r.All()
	if hooks[0].Status != StatusVerified {
		t.Errorf("expected verified, got %s", hooks[0].Status)
	}
}

func TestMigrate_NotPending_Error(t *testing.T) {
	r := NewRegistry()
	r.Register(HookRecord{Name: "h1", Status: StatusMigrated})
	if err := r.Migrate("h1"); err == nil {
		t.Error("expected error migrating already-migrated hook")
	}
}

func TestMigrate_NotFound_Error(t *testing.T) {
	r := NewRegistry()
	if err := r.Migrate("missing"); err == nil {
		t.Error("expected error for missing hook")
	}
}

func TestVerify_NotMigrated_Error(t *testing.T) {
	r := NewRegistry()
	r.Register(HookRecord{Name: "h1", Status: StatusPending})
	if err := r.Verify("h1"); err == nil {
		t.Error("expected error verifying unmigrated hook")
	}
}

func TestPendingCount(t *testing.T) {
	r := NewRegistry()
	r.Register(HookRecord{Name: "a", Status: StatusPending})
	r.Register(HookRecord{Name: "b", Status: StatusPending})
	r.Register(HookRecord{Name: "c", Status: StatusMigrated})
	if r.PendingCount() != 2 {
		t.Errorf("expected 2 pending, got %d", r.PendingCount())
	}
	r.Migrate("a")
	if r.PendingCount() != 1 {
		t.Errorf("expected 1 pending after migration, got %d", r.PendingCount())
	}
}
