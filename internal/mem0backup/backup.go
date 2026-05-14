package mem0backup

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const backupPattern = "mem0-backup-*.ndjson"
const dateFormat = "2006-01-02"

// FileExporter writes an NDJSON export to a file path.
type FileExporter interface {
	ExportToFile(path string) (int, error)
}

// BackupManager creates daily NDJSON backups and rotates old ones.
type BackupManager struct {
	BackupDir string
	Keep      int
	Exporter  FileExporter
	Now       func() time.Time
}

func (b *BackupManager) now() time.Time {
	if b.Now != nil {
		return b.Now()
	}
	return time.Now()
}

func (b *BackupManager) keep() int {
	if b.Keep > 0 {
		return b.Keep
	}
	return 7
}

// Run creates today's backup and rotates old ones.
// Returns the number of memories exported.
func (b *BackupManager) Run() (int, error) {
	if err := os.MkdirAll(b.BackupDir, 0o755); err != nil {
		return 0, fmt.Errorf("create backup dir: %w", err)
	}

	today := b.now().Format(dateFormat)
	filename := fmt.Sprintf("mem0-backup-%s.ndjson", today)
	target := filepath.Join(b.BackupDir, filename)

	tmpFile := target + ".tmp"
	n, err := b.Exporter.ExportToFile(tmpFile)
	if err != nil {
		os.Remove(tmpFile)
		return 0, fmt.Errorf("export: %w", err)
	}

	if err := os.Rename(tmpFile, target); err != nil {
		os.Remove(tmpFile)
		return 0, fmt.Errorf("rename backup: %w", err)
	}

	if err := b.rotate(); err != nil {
		return n, fmt.Errorf("rotate: %w", err)
	}

	return n, nil
}

func (b *BackupManager) rotate() error {
	matches, err := filepath.Glob(filepath.Join(b.BackupDir, backupPattern))
	if err != nil {
		return fmt.Errorf("glob backups: %w", err)
	}

	if len(matches) <= b.keep() {
		return nil
	}

	sort.Slice(matches, func(i, j int) bool {
		return extractDate(matches[i]) < extractDate(matches[j])
	})

	toDelete := matches[:len(matches)-b.keep()]
	for _, path := range toDelete {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove old backup %s: %w", filepath.Base(path), err)
		}
	}
	return nil
}

func extractDate(path string) string {
	base := filepath.Base(path)
	base = strings.TrimPrefix(base, "mem0-backup-")
	base = strings.TrimSuffix(base, ".ndjson")
	return base
}
