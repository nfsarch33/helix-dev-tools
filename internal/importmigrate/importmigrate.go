// Package importmigrate rewrites Go import paths across a module tree in a
// single atomic pass. It walks .go files concurrently, replaces all
// occurrences of OldPrefix with NewPrefix in each file, and writes the result
// back atomically (write-to-tmp then rename) so a partial failure cannot leave
// a file half-written.
//
// go.mod, vendor/, and .git/ are intentionally excluded from rewriting; the
// caller is responsible for patching go.mod separately.
package importmigrate

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
)

// Config parameterises a Migrate call.
type Config struct {
	// Root is the directory tree to walk.
	Root string
	// OldPrefix is the exact import-path prefix to replace.
	OldPrefix string
	// NewPrefix replaces every OldPrefix occurrence.
	NewPrefix string
	// DryRun counts what would be changed without writing to disk.
	DryRun bool
	// Concurrency sets the number of goroutines used for file I/O.
	// Defaults to GOMAXPROCS when zero or negative.
	Concurrency int
}

// Result summarises a completed Migrate call.
type Result struct {
	FilesScanned       int64
	FilesChanged       int64
	SubstitutionsTotal int64
}

// Summary returns a one-line human-readable report.
func (r Result) Summary() string {
	return fmt.Sprintf("scanned %d files, changed %d, %d substitutions total",
		r.FilesScanned, r.FilesChanged, r.SubstitutionsTotal)
}

// Migrate walks Root, replacing every occurrence of Config.OldPrefix with
// Config.NewPrefix in .go files. It returns a Result with change counts and
// the first error encountered (if any). Files in vendor/, .git/, and
// node_modules/ are skipped.
func Migrate(cfg Config) (Result, error) {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = runtime.GOMAXPROCS(0)
	}

	type job struct{ path string }

	jobs := make(chan job, cfg.Concurrency*4)
	errs := make(chan error, 1)

	var (
		result    Result
		wg        sync.WaitGroup
		firstErr  error
		errMu     sync.Mutex
	)

	sendErr := func(err error) {
		errMu.Lock()
		if firstErr == nil {
			firstErr = err
		}
		errMu.Unlock()
	}

	// Start worker pool.
	for range cfg.Concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				changed, subs, err := processFile(j.path, cfg.OldPrefix, cfg.NewPrefix, cfg.DryRun)
				if err != nil {
					sendErr(err)
					return
				}
				atomic.AddInt64(&result.FilesScanned, 1)
				if changed {
					atomic.AddInt64(&result.FilesChanged, 1)
					atomic.AddInt64(&result.SubstitutionsTotal, int64(subs))
				}
			}
		}()
	}

	// Walk the tree and feed jobs; errors are sent on the errs channel.
	go func() {
		defer close(jobs)
		err := filepath.WalkDir(cfg.Root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				name := d.Name()
				if name == "vendor" || name == ".git" || name == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}
			if filepath.Ext(path) != ".go" {
				return nil
			}
			jobs <- job{path}
			return nil
		})
		if err != nil {
			select {
			case errs <- err:
			default:
			}
		}
	}()

	wg.Wait()

	select {
	case walkErr := <-errs:
		if firstErr == nil {
			firstErr = walkErr
		}
	default:
	}

	return result, firstErr
}

// processFile replaces OldPrefix with NewPrefix in the named file.
// Returns (changed bool, substitution count, error).
// When dryRun is true the file is read and counted but not written.
func processFile(path, oldPrefix, newPrefix string, dryRun bool) (bool, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, 0, fmt.Errorf("read %s: %w", path, err)
	}

	oldB := []byte(oldPrefix)
	newB := []byte(newPrefix)

	count := bytes.Count(data, oldB)
	if count == 0 {
		return false, 0, nil
	}

	replaced := bytes.ReplaceAll(data, oldB, newB)

	if dryRun {
		return true, count, nil
	}

	// Atomic write: write to a temp file in the same directory, then rename.
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".importmigrate-*")
	if err != nil {
		return false, 0, fmt.Errorf("create temp %s: %w", dir, err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(replaced); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return false, 0, fmt.Errorf("write temp %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return false, 0, fmt.Errorf("close temp %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return false, 0, fmt.Errorf("rename %s -> %s: %w", tmpName, path, err)
	}

	return true, count, nil
}

