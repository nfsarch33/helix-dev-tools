package deploymgr

import (
	"os"
	"testing"
)

func TestRegister_StoresRecord(t *testing.T) {
	m := NewManager()
	m.Register(BinaryRecord{Name: "cursor-tools", Version: "v9.0.0", Path: "/bin/ct"})
	r, ok := m.Latest("cursor-tools")
	if !ok {
		t.Fatal("expected record to exist")
	}
	if r.Version != "v9.0.0" {
		t.Errorf("expected v9.0.0, got %s", r.Version)
	}
}

func TestRegister_SetsDeployedAt(t *testing.T) {
	m := NewManager()
	m.Register(BinaryRecord{Name: "ct", Version: "v1"})
	r, _ := m.Latest("ct")
	if r.DeployedAt.IsZero() {
		t.Error("expected DeployedAt to be set")
	}
}

func TestLatest_ReturnsNewest(t *testing.T) {
	m := NewManager()
	m.Register(BinaryRecord{Name: "ct", Version: "v1"})
	m.Register(BinaryRecord{Name: "ct", Version: "v2"})
	r, _ := m.Latest("ct")
	if r.Version != "v2" {
		t.Errorf("expected v2, got %s", r.Version)
	}
}

func TestPrevious_ReturnsRollback(t *testing.T) {
	m := NewManager()
	m.Register(BinaryRecord{Name: "ct", Version: "v1"})
	m.Register(BinaryRecord{Name: "ct", Version: "v2"})
	r, ok := m.Previous("ct")
	if !ok {
		t.Fatal("expected previous record")
	}
	if r.Version != "v1" {
		t.Errorf("expected v1, got %s", r.Version)
	}
}

func TestPrevious_NotFound_SingleRecord(t *testing.T) {
	m := NewManager()
	m.Register(BinaryRecord{Name: "ct", Version: "v1"})
	_, ok := m.Previous("ct")
	if ok {
		t.Error("expected no previous for single record")
	}
}

func TestAll_ReturnsHistory(t *testing.T) {
	m := NewManager()
	m.Register(BinaryRecord{Name: "ct", Version: "v1"})
	m.Register(BinaryRecord{Name: "ct", Version: "v2"})
	m.Register(BinaryRecord{Name: "other", Version: "v1"})
	all := m.All("ct")
	if len(all) != 2 {
		t.Errorf("expected 2 records, got %d", len(all))
	}
}

func TestHashFile(t *testing.T) {
	f, err := os.CreateTemp("", "hash_test_*.txt")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer os.Remove(f.Name())
	f.WriteString("hello world")
	f.Close()

	hash, err := HashFile(f.Name())
	if err != nil {
		t.Fatalf("HashFile: %v", err)
	}
	if len(hash) != 64 {
		t.Errorf("expected 64-char hex hash, got len=%d", len(hash))
	}
}
