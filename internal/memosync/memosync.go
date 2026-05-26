package memosync

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SyncDirs maps source subdirectories in global-kb to destination
// subdirectories in the memo repo.
var SyncDirs = map[string]string{
	"sop":              "sop",
	"adrs":             "adrs",
	"cursor-config":    "config",
	"session-handoffs": "handoffs",
}

// Result captures the outcome of a sync operation.
type Result struct {
	Copied  []string
	Skipped int
	Errors  []error
}

// Sync copies changed files from globalKBRoot into memoRoot according
// to SyncDirs. A file is copied only when its content hash differs from
// the destination (or the destination does not exist).
func Sync(globalKBRoot, memoRoot string) (*Result, error) {
	if err := validateDir(globalKBRoot); err != nil {
		return nil, fmt.Errorf("global-kb root: %w", err)
	}
	if err := validateDir(memoRoot); err != nil {
		return nil, fmt.Errorf("memo root: %w", err)
	}

	res := &Result{}
	for srcSub, dstSub := range SyncDirs {
		srcDir := filepath.Join(globalKBRoot, srcSub)
		dstDir := filepath.Join(memoRoot, dstSub)

		if _, err := os.Stat(srcDir); os.IsNotExist(err) {
			continue
		}

		if err := os.MkdirAll(dstDir, 0o755); err != nil {
			res.Errors = append(res.Errors, fmt.Errorf("mkdir %s: %w", dstSub, err))
			continue
		}

		err := filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(srcDir, path)
			dst := filepath.Join(dstDir, rel)

			changed, copyErr := fileChanged(path, dst)
			if copyErr != nil {
				res.Errors = append(res.Errors, copyErr)
				return nil
			}
			if !changed {
				res.Skipped++
				return nil
			}

			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				res.Errors = append(res.Errors, err)
				return nil
			}
			if err := copyFile(path, dst); err != nil {
				res.Errors = append(res.Errors, fmt.Errorf("copy %s: %w", rel, err))
				return nil
			}
			res.Copied = append(res.Copied, filepath.Join(dstSub, rel))
			return nil
		})
		if err != nil {
			res.Errors = append(res.Errors, fmt.Errorf("walk %s: %w", srcSub, err))
		}
	}
	return res, nil
}

// CommitMessage returns a conventional commit message for a sync run.
func CommitMessage() string {
	return fmt.Sprintf("sync: %s", time.Now().Format(time.RFC3339))
}

func validateDir(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", path)
	}
	return nil
}

func fileChanged(src, dst string) (bool, error) {
	dstInfo, err := os.Stat(dst)
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	srcInfo, err := os.Stat(src)
	if err != nil {
		return false, err
	}
	if srcInfo.Size() != dstInfo.Size() {
		return true, nil
	}
	srcHash, err := hashFile(src)
	if err != nil {
		return false, err
	}
	dstHash, err := hashFile(dst)
	if err != nil {
		return false, err
	}
	return srcHash != dstHash, nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

// UpdateReadme rewrites the README.md in memoRoot with the last-sync
// timestamp. Returns true if the file was modified.
func UpdateReadme(memoRoot string, syncTime time.Time) (bool, error) {
	readmePath := filepath.Join(memoRoot, "README.md")
	data, err := os.ReadFile(readmePath)
	if err != nil {
		return false, nil
	}
	content := string(data)
	marker := "Last sync: "
	idx := strings.Index(content, marker)
	if idx < 0 {
		return false, nil
	}
	newLine := marker + syncTime.Format(time.RFC3339)
	lineEnd := strings.Index(content[idx:], "\n")
	var updated string
	if lineEnd >= 0 {
		updated = content[:idx] + newLine + content[idx+lineEnd:]
	} else {
		updated = content[:idx] + newLine
	}
	if updated == content {
		return false, nil
	}
	return true, os.WriteFile(readmePath, []byte(updated), 0o644)
}
