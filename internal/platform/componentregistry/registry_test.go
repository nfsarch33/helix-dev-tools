package componentregistry

import (
	"testing"
	"time"
)

func TestRegisterAndLookup(t *testing.T) {
	r := New()

	comp := Component{
		ID:          "mem0-api",
		Name:        "Mem0 OSS API",
		Category:    CategoryService,
		InstallPath: "/home/user/ops/deploy/mem0-selfhost/",
		ConfigPath:  "/home/user/ops/deploy/mem0-selfhost/docker-compose.yml",
		HealthCheck: "curl -sS http://localhost:8888/healthz",
		Owner:       "cursor-parent",
		Node:        "gpu-host-1",
	}

	err := r.Register(comp)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	got, err := r.Lookup("mem0-api")
	if err != nil {
		t.Fatalf("Lookup failed: %v", err)
	}
	if got.Name != "Mem0 OSS API" {
		t.Errorf("got name %q, want %q", got.Name, "Mem0 OSS API")
	}
	if got.Category != CategoryService {
		t.Errorf("got category %q, want %q", got.Category, CategoryService)
	}
}

func TestRegisterDuplicate(t *testing.T) {
	r := New()

	comp := Component{
		ID:       "vllm",
		Name:     "vLLM Inference",
		Category: CategoryDaemon,
	}

	if err := r.Register(comp); err != nil {
		t.Fatalf("first Register: %v", err)
	}

	err := r.Register(comp)
	if err == nil {
		t.Fatal("expected error for duplicate registration")
	}
}

func TestLookupNotFound(t *testing.T) {
	r := New()

	_, err := r.Lookup("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing component")
	}
}

func TestListByCategory(t *testing.T) {
	r := New()

	r.Register(Component{ID: "mem0-api", Category: CategoryService})
	r.Register(Component{ID: "vllm", Category: CategoryDaemon})
	r.Register(Component{ID: "router", Category: CategoryDaemon})
	r.Register(Component{ID: "k3s", Category: CategoryInfra})

	services := r.ListByCategory(CategoryService)
	if len(services) != 1 {
		t.Errorf("expected 1 service, got %d", len(services))
	}

	daemons := r.ListByCategory(CategoryDaemon)
	if len(daemons) != 2 {
		t.Errorf("expected 2 daemons, got %d", len(daemons))
	}
}

func TestListByNode(t *testing.T) {
	r := New()

	r.Register(Component{ID: "mem0-api", Node: "gpu-host-1"})
	r.Register(Component{ID: "vllm", Node: "gpu-host-1"})
	r.Register(Component{ID: "dashboard", Node: "gpu-host-2"})

	host1 := r.ListByNode("gpu-host-1")
	if len(host1) != 2 {
		t.Errorf("expected 2 on gpu-host-1, got %d", len(host1))
	}

	host2 := r.ListByNode("gpu-host-2")
	if len(host2) != 1 {
		t.Errorf("expected 1 on gpu-host-2, got %d", len(host2))
	}
}

func TestAll(t *testing.T) {
	r := New()

	r.Register(Component{ID: "a"})
	r.Register(Component{ID: "b"})
	r.Register(Component{ID: "c"})

	all := r.All()
	if len(all) != 3 {
		t.Errorf("expected 3, got %d", len(all))
	}
}

func TestDeregister(t *testing.T) {
	r := New()

	r.Register(Component{ID: "temp"})

	err := r.Deregister("temp")
	if err != nil {
		t.Fatalf("Deregister failed: %v", err)
	}

	_, err = r.Lookup("temp")
	if err == nil {
		t.Fatal("expected error after deregister")
	}
}

func TestDeregisterNotFound(t *testing.T) {
	r := New()

	err := r.Deregister("ghost")
	if err == nil {
		t.Fatal("expected error for deregistering nonexistent component")
	}
}

func TestUpdateStatus(t *testing.T) {
	r := New()

	r.Register(Component{ID: "svc", Status: StatusUnknown})

	err := r.UpdateStatus("svc", StatusHealthy)
	if err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	got, _ := r.Lookup("svc")
	if got.Status != StatusHealthy {
		t.Errorf("got status %q, want %q", got.Status, StatusHealthy)
	}
	if got.LastChecked.IsZero() {
		t.Error("LastChecked should be set after status update")
	}
}

func TestValidateComponent(t *testing.T) {
	tests := []struct {
		name    string
		comp    Component
		wantErr bool
	}{
		{"valid", Component{ID: "x", Name: "X", Category: CategoryService}, false},
		{"empty id", Component{ID: "", Name: "X", Category: CategoryService}, true},
		{"empty name", Component{ID: "x", Name: "", Category: CategoryService}, true},
		{"invalid category", Component{ID: "x", Name: "X", Category: "bogus"}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.comp)
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestSnapshot(t *testing.T) {
	r := New()

	r.Register(Component{ID: "a", Status: StatusHealthy})
	r.Register(Component{ID: "b", Status: StatusDegraded})
	r.Register(Component{ID: "c", Status: StatusDown})

	snap := r.Snapshot()
	if snap.Total != 3 {
		t.Errorf("expected total 3, got %d", snap.Total)
	}
	if snap.Healthy != 1 {
		t.Errorf("expected 1 healthy, got %d", snap.Healthy)
	}
	if snap.Degraded != 1 {
		t.Errorf("expected 1 degraded, got %d", snap.Degraded)
	}
	if snap.Down != 1 {
		t.Errorf("expected 1 down, got %d", snap.Down)
	}
	if snap.Timestamp.After(time.Now()) {
		t.Error("snapshot timestamp should not be in the future")
	}
}
