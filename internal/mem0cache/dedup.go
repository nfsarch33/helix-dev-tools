package mem0cache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	bolt "go.etcd.io/bbolt"
)

var dedupBucket = []byte("dedup")

// DedupStats reports dedup index state.
type DedupStats struct {
	TotalEntries int
	Hits         int64
	Misses       int64
}

// Dedup tracks content hashes to prevent duplicate Mem0 adds.
type Dedup struct {
	db     *bolt.DB
	mu     sync.Mutex
	hits   int64
	misses int64
}

// DedupConfig controls the dedup store.
type DedupConfig struct {
	DBPath string // defaults to ~/.config/cursor-tools/mem0-dedup.db
}

func defaultDedupDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	dir := filepath.Join(home, ".config", "cursor-tools")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}
	return filepath.Join(dir, "mem0-dedup.db"), nil
}

// NewDedup opens (or creates) the dedup BoltDB store.
func NewDedup(cfg DedupConfig) (*Dedup, error) {
	dbPath := cfg.DBPath
	if dbPath == "" {
		var err error
		dbPath, err = defaultDedupDBPath()
		if err != nil {
			return nil, err
		}
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	db, err := bolt.Open(dbPath, 0o600, &bolt.Options{Timeout: 2e9})
	if err != nil {
		return nil, fmt.Errorf("open dedup db: %w", err)
	}

	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(dedupBucket)
		return err
	}); err != nil {
		db.Close()
		return nil, fmt.Errorf("create dedup bucket: %w", err)
	}

	return &Dedup{db: db}, nil
}

func contentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

// IsDuplicate returns true if the content has already been marked.
func (d *Dedup) IsDuplicate(content string) bool {
	hash := contentHash(content)
	var found bool

	_ = d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(dedupBucket)
		if b == nil {
			return nil
		}
		found = b.Get([]byte(hash)) != nil
		return nil
	})

	d.mu.Lock()
	if found {
		d.hits++
	} else {
		d.misses++
	}
	d.mu.Unlock()
	if found {
		slog.Debug("dedup hit", "hash", hash[:12])
	}
	return found
}

// Mark records the content hash so future IsDuplicate calls return true.
func (d *Dedup) Mark(content string) error {
	hash := contentHash(content)
	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(dedupBucket)
		if b == nil {
			return fmt.Errorf("dedup bucket missing")
		}
		return b.Put([]byte(hash), []byte("1"))
	})
}

// Stats returns dedup statistics.
func (d *Dedup) Stats() DedupStats {
	var count int
	_ = d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(dedupBucket)
		if b != nil {
			count = b.Stats().KeyN
		}
		return nil
	})
	d.mu.Lock()
	defer d.mu.Unlock()
	return DedupStats{
		TotalEntries: count,
		Hits:         d.hits,
		Misses:       d.misses,
	}
}

// Close shuts down the BoltDB store.
func (d *Dedup) Close() error {
	return d.db.Close()
}
