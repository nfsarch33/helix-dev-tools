package devex

import (
	"fmt"
	"sort"
)

type Migration struct {
	Version int
	Name    string
	Up      func(config map[string]interface{}) (map[string]interface{}, error)
}

type Migrator struct {
	migrations []Migration
}

func NewMigrator() *Migrator {
	return &Migrator{}
}

func (m *Migrator) Register(migration Migration) {
	m.migrations = append(m.migrations, migration)
	sort.Slice(m.migrations, func(i, j int) bool {
		return m.migrations[i].Version < m.migrations[j].Version
	})
}

func (m *Migrator) Migrate(config map[string]interface{}, fromVersion, toVersion int) (map[string]interface{}, error) {
	current := copyMap(config)

	for _, mig := range m.migrations {
		if mig.Version <= fromVersion {
			continue
		}
		if mig.Version > toVersion {
			break
		}
		var err error
		current, err = mig.Up(current)
		if err != nil {
			return nil, fmt.Errorf("migration v%d (%s) failed: %w", mig.Version, mig.Name, err)
		}
	}
	return current, nil
}

func (m *Migrator) LatestVersion() int {
	if len(m.migrations) == 0 {
		return 0
	}
	return m.migrations[len(m.migrations)-1].Version
}

func (m *Migrator) MigrationCount() int {
	return len(m.migrations)
}

func (m *Migrator) Pending(currentVersion int) []Migration {
	var pending []Migration
	for _, mig := range m.migrations {
		if mig.Version > currentVersion {
			pending = append(pending, mig)
		}
	}
	return pending
}

func copyMap(src map[string]interface{}) map[string]interface{} {
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
