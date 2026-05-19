package devex

import (
	"errors"
	"testing"
)

func TestMigrator_SingleMigration(t *testing.T) {
	m := NewMigrator()
	m.Register(Migration{
		Version: 2,
		Name:    "add-timeout",
		Up: func(cfg map[string]interface{}) (map[string]interface{}, error) {
			cfg["timeout"] = 30
			return cfg, nil
		},
	})

	result, err := m.Migrate(map[string]interface{}{"host": "localhost"}, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if result["timeout"] != 30 {
		t.Error("expected timeout=30 after migration")
	}
}

func TestMigrator_SkipOldMigrations(t *testing.T) {
	m := NewMigrator()
	m.Register(Migration{Version: 1, Name: "old", Up: func(cfg map[string]interface{}) (map[string]interface{}, error) {
		cfg["old"] = true
		return cfg, nil
	}})
	m.Register(Migration{Version: 3, Name: "new", Up: func(cfg map[string]interface{}) (map[string]interface{}, error) {
		cfg["new"] = true
		return cfg, nil
	}})

	result, err := m.Migrate(map[string]interface{}{}, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := result["old"]; ok {
		t.Error("old migration should have been skipped")
	}
	if result["new"] != true {
		t.Error("new migration should have run")
	}
}

func TestMigrator_FailedMigration(t *testing.T) {
	m := NewMigrator()
	m.Register(Migration{Version: 1, Name: "fail", Up: func(cfg map[string]interface{}) (map[string]interface{}, error) {
		return nil, errors.New("broken")
	}})

	_, err := m.Migrate(map[string]interface{}{}, 0, 1)
	if err == nil {
		t.Error("expected error from failed migration")
	}
}

func TestMigrator_LatestVersion(t *testing.T) {
	m := NewMigrator()
	m.Register(Migration{Version: 5})
	m.Register(Migration{Version: 3})

	if m.LatestVersion() != 5 {
		t.Errorf("expected 5, got %d", m.LatestVersion())
	}
}

func TestMigrator_Pending(t *testing.T) {
	m := NewMigrator()
	m.Register(Migration{Version: 1})
	m.Register(Migration{Version: 2})
	m.Register(Migration{Version: 3})

	pending := m.Pending(1)
	if len(pending) != 2 {
		t.Errorf("expected 2 pending, got %d", len(pending))
	}
}
