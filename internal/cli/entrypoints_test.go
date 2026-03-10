package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nfsarch33/cursor-tools/internal/patterns"
)

func TestHookEntrypointsAndSmallWrappers(t *testing.T) {
	t.Run("runGuardMcp denies dangerous tools", func(t *testing.T) {
		oldExit := guardMcpExit
		oldHome := os.Getenv("HOME")
		home := t.TempDir()
		if err := os.Setenv("HOME", home); err != nil {
			t.Fatal(err)
		}
		defer func() {
			guardMcpExit = oldExit
			_ = os.Setenv("HOME", oldHome)
		}()
		exitCalled := false
		guardMcpExit = func(code int) {
			exitCalled = true
			panic(code)
		}

		inFile := filepath.Join(home, "mcp-input.json")
		outFile := filepath.Join(home, "mcp-output.json")
		if err := os.WriteFile(inFile, []byte(`{"tool_name":"delete_issue","tool_input":"{}"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		in, _ := os.Open(inFile)
		defer in.Close()
		out, _ := os.Create(outFile)
		defer out.Close()

		defer func() {
			if recover() == nil {
				t.Fatal("expected guardMcpExit panic")
			}
			if !exitCalled {
				t.Fatal("guardMcpExit not called")
			}
		}()
		_ = runGuardMcp(in, out)
	})

	t.Run("runGuardMcp allows safe tools and bad input", func(t *testing.T) {
		oldHome := os.Getenv("HOME")
		home := t.TempDir()
		if err := os.Setenv("HOME", home); err != nil {
			t.Fatal(err)
		}
		defer os.Setenv("HOME", oldHome)

		for name, payload := range map[string]string{
			"safe": `{"tool_name":"search","tool_input":"{}"}`,
			"bad":  `not-json`,
		} {
			inFile := filepath.Join(home, name+".json")
			outFile := filepath.Join(home, name+"-out.json")
			if err := os.WriteFile(inFile, []byte(payload), 0o644); err != nil {
				t.Fatal(err)
			}
			in, _ := os.Open(inFile)
			defer in.Close()
			out, _ := os.Create(outFile)
			defer out.Close()
			if err := runGuardMcp(in, out); err != nil {
				t.Fatalf("runGuardMcp(%s) error = %v", name, err)
			}
			data, err := os.ReadFile(outFile)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), `"permission":"allow"`) {
				t.Fatalf("runGuardMcp(%s) output = %q", name, string(data))
			}
		}
	})

	t.Run("newGuardShellHandler and runGuardShell work", func(t *testing.T) {
		oldExit := guardShellExit
		oldHome := os.Getenv("HOME")
		home := t.TempDir()
		if err := os.Setenv("HOME", home); err != nil {
			t.Fatal(err)
		}
		defer func() {
			guardShellExit = oldExit
			_ = os.Setenv("HOME", oldHome)
		}()
		h, err := newGuardShellHandler()
		if err != nil {
			t.Fatalf("newGuardShellHandler() error = %v", err)
		}
		if h.matcher == nil {
			t.Fatal("newGuardShellHandler() matcher is nil")
		}

		exitCalled := false
		guardShellExit = func(code int) {
			exitCalled = true
			panic(code)
		}
		inFile := filepath.Join(home, "shell-input.json")
		outFile := filepath.Join(home, "shell-output.json")
		if err := os.WriteFile(inFile, []byte(`{"command":"rm -rf /"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		in, _ := os.Open(inFile)
		defer in.Close()
		out, _ := os.Create(outFile)
		defer out.Close()

		defer func() {
			if recover() == nil {
				t.Fatal("expected guardShellExit panic")
			}
			if !exitCalled {
				t.Fatal("guardShellExit not called")
			}
		}()
		_ = runGuardShell(in, out)
	})

	t.Run("runGuardShell allows safe commands and invalid input", func(t *testing.T) {
		oldHome := os.Getenv("HOME")
		home := t.TempDir()
		if err := os.Setenv("HOME", home); err != nil {
			t.Fatal(err)
		}
		defer os.Setenv("HOME", oldHome)

		for name, payload := range map[string]string{
			"safe": `{"command":"git status"}`,
			"bad":  `not-json`,
		} {
			inFile := filepath.Join(home, name+".json")
			outFile := filepath.Join(home, name+"-out.json")
			if err := os.WriteFile(inFile, []byte(payload), 0o644); err != nil {
				t.Fatal(err)
			}
			in, _ := os.Open(inFile)
			defer in.Close()
			out, _ := os.Create(outFile)
			defer out.Close()
			if err := runGuardShell(in, out); err != nil {
				t.Fatalf("runGuardShell(%s) error = %v", name, err)
			}
			data, err := os.ReadFile(outFile)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), `"permission":"allow"`) {
				t.Fatalf("runGuardShell(%s) output = %q", name, string(data))
			}
		}
	})

	t.Run("runSanitizeRead denies blocked files", func(t *testing.T) {
		oldExit := sanitizeReadExit
		oldHome := os.Getenv("HOME")
		home := t.TempDir()
		if err := os.Setenv("HOME", home); err != nil {
			t.Fatal(err)
		}
		defer func() {
			sanitizeReadExit = oldExit
			_ = os.Setenv("HOME", oldHome)
		}()
		exitCalled := false
		sanitizeReadExit = func(code int) {
			exitCalled = true
			panic(code)
		}

		inFile := filepath.Join(home, "read-input.json")
		outFile := filepath.Join(home, "read-output.json")
		if err := os.WriteFile(inFile, []byte(`{"file_path":"/tmp/.env"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		in, _ := os.Open(inFile)
		defer in.Close()
		out, _ := os.Create(outFile)
		defer out.Close()

		defer func() {
			if recover() == nil {
				t.Fatal("expected sanitizeReadExit panic")
			}
			if !exitCalled {
				t.Fatal("sanitizeReadExit not called")
			}
		}()
		_ = runSanitizeRead(in, out)
	})

	t.Run("runSanitizeRead allows safe files and invalid input", func(t *testing.T) {
		oldHome := os.Getenv("HOME")
		home := t.TempDir()
		if err := os.Setenv("HOME", home); err != nil {
			t.Fatal(err)
		}
		defer os.Setenv("HOME", oldHome)

		for name, payload := range map[string]string{
			"safe": `{"file_path":"/tmp/readme.md"}`,
			"bad":  `not-json`,
		} {
			inFile := filepath.Join(home, name+".json")
			outFile := filepath.Join(home, name+"-out.json")
			if err := os.WriteFile(inFile, []byte(payload), 0o644); err != nil {
				t.Fatal(err)
			}
			in, _ := os.Open(inFile)
			defer in.Close()
			out, _ := os.Create(outFile)
			defer out.Close()
			if err := runSanitizeRead(in, out); err != nil {
				t.Fatalf("runSanitizeRead(%s) error = %v", name, err)
			}
			data, err := os.ReadFile(outFile)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), `"permission":"allow"`) {
				t.Fatalf("runSanitizeRead(%s) output = %q", name, string(data))
			}
		}
	})

	t.Run("runSelftest passes", func(t *testing.T) {
		if err := runSelftest(nil, nil); err != nil {
			t.Fatalf("runSelftest() error = %v", err)
		}
	})

	t.Run("resolveOpenclawDir prefers flag", func(t *testing.T) {
		old := openclawDir
		defer func() { openclawDir = old }()
		openclawDir = "/tmp/openclaw"
		if got := resolveOpenclawDir(); got != "/tmp/openclaw" {
			t.Fatalf("resolveOpenclawDir() = %q", got)
		}
	})

	t.Run("Execute runs version command", func(t *testing.T) {
		rootCmd.SetArgs([]string{"version"})
		if err := Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		rootCmd.SetArgs(nil)
	})

	t.Run("gitRepoRoot uses git output", func(t *testing.T) {
		binDir := t.TempDir()
		restorePath := prependPath(t, binDir)
		defer restorePath()
		writeExecutable(t, binDir, "git", "#!/bin/sh\necho /tmp/repo\n")
		root, err := gitRepoRoot()
		if err != nil {
			t.Fatalf("gitRepoRoot() error = %v", err)
		}
		if root != "/tmp/repo" {
			t.Fatalf("gitRepoRoot() = %q, want /tmp/repo", root)
		}
	})

	t.Run("pattern matcher still compiles", func(t *testing.T) {
		m, err := patterns.NewMatcher(patterns.ShellDenyPatterns, patterns.ShellWarnPatterns)
		if err != nil {
			t.Fatalf("patterns.NewMatcher() error = %v", err)
		}
		action, _ := m.Match("git status")
		if action != patterns.ActionAllow {
			t.Fatalf("matcher action = %d, want allow", action)
		}
	})

}
