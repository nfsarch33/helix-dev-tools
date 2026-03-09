package worktree

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// WorktreeDir is the conventional directory for linked worktrees.
const WorktreeDir = ".worktrees"

// Entry represents a single git worktree.
type Entry struct {
	Path   string
	Commit string
	Branch string
}

// EnsureGitignore adds pattern to .gitignore in repoRoot if not already present.
func EnsureGitignore(repoRoot, pattern string) error {
	gitignorePath := filepath.Join(repoRoot, ".gitignore")

	f, err := os.OpenFile(gitignorePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("open .gitignore: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == strings.TrimSpace(pattern) {
			return nil
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read .gitignore: %w", err)
	}

	if _, err := f.WriteString(pattern + "\n"); err != nil {
		return fmt.Errorf("write .gitignore: %w", err)
	}
	return nil
}

// CopyEnvFiles copies all .env* files from src to dst.
// Returns a slice of filenames that were copied.
func CopyEnvFiles(src, dst string) ([]string, error) {
	entries, err := os.ReadDir(src)
	if err != nil {
		return nil, fmt.Errorf("readdir %s: %w", src, err)
	}

	var copied []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, ".env") {
			continue
		}

		srcPath := filepath.Join(src, name)
		dstPath := filepath.Join(dst, name)

		if cpErr := copyFile(srcPath, dstPath); cpErr != nil {
			return copied, fmt.Errorf("copy %s: %w", name, cpErr)
		}
		copied = append(copied, name)
	}
	return copied, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

var branchRe = regexp.MustCompile(`\[(.+?)\]`)

// ParseWorktreeList parses the output of `git worktree list` into structured entries.
func ParseWorktreeList(output string) []Entry {
	var entries []Entry
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		path := fields[0]
		commit := fields[1]
		branch := ""

		if m := branchRe.FindStringSubmatch(line); len(m) > 1 {
			branch = m[1]
		}

		entries = append(entries, Entry{
			Path:   path,
			Commit: commit,
			Branch: branch,
		})
	}
	return entries
}
