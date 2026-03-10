package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const helperEnv = "CURSOR_TOOLS_TEST_HELPER"

func TestMain(m *testing.M) {
	if os.Getenv(helperEnv) != "" {
		runTestHelper()
		return
	}
	os.Exit(m.Run())
}

func runTestHelper() {
	args := nonTestArgs(os.Args[1:])
	if len(args) == 0 {
		os.Exit(0)
	}

	cmd := args[0]
	helperLog(fmt.Sprintf("%s %s", cmd, strings.Join(args[1:], " ")))

	failSet := map[string]bool{}
	for _, name := range strings.Split(os.Getenv("CURSOR_TOOLS_HELPER_FAIL"), ",") {
		name = strings.TrimSpace(name)
		if name != "" {
			failSet[name] = true
		}
	}

	if failSet[cmd] {
		fmt.Fprintln(os.Stderr, "helper failure:", cmd)
		os.Exit(1)
	}

	switch cmd {
	case "sync-counts", "promote":
		fmt.Fprintln(os.Stdout, "helper ok:", cmd)
	default:
		fmt.Fprintln(os.Stdout, "helper unknown:", cmd)
	}
	os.Exit(0)
}

func nonTestArgs(args []string) []string {
	filtered := make([]string, 0, len(args))
	for _, arg := range args {
		if strings.HasPrefix(arg, "-test.") {
			continue
		}
		filtered = append(filtered, arg)
	}
	return filtered
}

func helperLog(line string) {
	path := os.Getenv("CURSOR_TOOLS_HELPER_LOG")
	if path == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = fmt.Fprintln(f, line)
}

func writeExecutable(t testing.TB, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", name, err)
	}
	return path
}

func prependPath(t testing.TB, dir string) func() {
	t.Helper()
	oldPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", dir+string(os.PathListSeparator)+oldPath); err != nil {
		t.Fatalf("set PATH: %v", err)
	}
	return func() {
		_ = os.Setenv("PATH", oldPath)
	}
}
