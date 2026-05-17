package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/config"
	"github.com/nfsarch33/helix-dev-tools/internal/hookio"
	"github.com/nfsarch33/helix-dev-tools/internal/logger"
	"github.com/nfsarch33/helix-dev-tools/internal/metrics"
)

func TestPostEditHandlerFormattersAndCodegen(t *testing.T) {
	oldHome := os.Getenv("HOME")
	home := t.TempDir()
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatal(err)
	}
	defer os.Setenv("HOME", oldHome)

	binDir := t.TempDir()
	logPath := filepath.Join(binDir, "commands.log")
	restorePath := prependPath(t, binDir)
	defer restorePath()

	writeExecutable(t, binDir, "gofmt", "#!/bin/sh\necho \"gofmt:$@\" >> \""+logPath+"\"\n")
	writeExecutable(t, binDir, "ruff", "#!/bin/sh\necho \"ruff:$@\" >> \""+logPath+"\"\n")
	writeExecutable(t, binDir, "dart", "#!/bin/sh\necho \"dart:$@\" >> \""+logPath+"\"\n")
	writeExecutable(t, binDir, "make", "#!/bin/sh\necho \"make:$@\" >> \""+logPath+"\"\n")
	writeExecutable(t, binDir, "git", "#!/bin/sh\nif [ \"$1\" = \"rev-parse\" ]; then\n  echo \"$TEST_GIT_ROOT\"\nfi\n")

	p := config.DefaultPaths()
	if err := os.MkdirAll(p.HooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	h := &postEditHandler{
		log:   logger.New(filepath.Join(home, "post-edit.log")),
		paths: p,
	}

	goFile := filepath.Join(home, "main.go")
	pyFile := filepath.Join(home, "main.py")
	dartFile := filepath.Join(home, "main.dart")
	jsonFile := filepath.Join(home, "data.json")
	for _, file := range []string{goFile, pyFile, dartFile} {
		if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(jsonFile, []byte("{\"a\":1}"), 0o644); err != nil {
		t.Fatal(err)
	}

	h.formatFile(goFile)
	h.formatFile(pyFile)
	h.formatFile(dartFile)
	h.formatFile(jsonFile)

	repoRoot := filepath.Join(home, "repo")
	if err := os.MkdirAll(repoRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("TEST_GIT_ROOT", repoRoot); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "Makefile"), []byte("schema-pdg:\n\t@echo ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	graphqlFile := filepath.Join(repoRoot, "schema.graphql")
	if err := os.WriteFile(graphqlFile, []byte("type Query { ok: Boolean }"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.runGraphQLCodegen(graphqlFile)

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	logText := string(logData)
	for _, want := range []string{"gofmt:-w " + goFile, "ruff:format " + pyFile, "dart:format " + dartFile, "make:schema-pdg"} {
		if !strings.Contains(logText, want) {
			t.Fatalf("command log missing %q in %q", want, logText)
		}
	}

	jsonData, err := os.ReadFile(jsonFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(jsonData)) != "{\n  \"a\": 1\n}" {
		t.Fatalf("json file = %q, want pretty JSON", string(jsonData))
	}
}

func TestPostEditSyncCountsAndPromoteLearnings(t *testing.T) {
	oldHome := os.Getenv("HOME")
	oldHelper := os.Getenv(helperEnv)
	oldHelperLog := os.Getenv("CURSOR_TOOLS_HELPER_LOG")
	home := t.TempDir()
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv(helperEnv, "1"); err != nil {
		t.Fatal(err)
	}
	helperLogPath := filepath.Join(home, "helper.log")
	if err := os.Setenv("CURSOR_TOOLS_HELPER_LOG", helperLogPath); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Setenv("HOME", oldHome)
		_ = os.Setenv(helperEnv, oldHelper)
		_ = os.Setenv("CURSOR_TOOLS_HELPER_LOG", oldHelperLog)
	}()

	p := config.DefaultPaths()
	if err := os.MkdirAll(p.HooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	h := &postEditHandler{
		log:   logger.New(filepath.Join(home, "post-edit.log")),
		paths: p,
	}

	h.syncCountsIfNeeded(filepath.Join(home, "skills-index.md"))

	workspace := filepath.Join(home, "workspace")
	if err := os.MkdirAll(filepath.Join(workspace, ".learnings"), 0o755); err != nil {
		t.Fatal(err)
	}
	h.promoteLearningsIfNeeded(filepath.Join(workspace, ".learnings", "LEARNINGS.md"))
	h.promoteLearningsIfNeeded(filepath.Join(home, "memo", "learnings", "PATTERNS.md"))

	logData, err := os.ReadFile(helperLogPath)
	if err != nil {
		t.Fatal(err)
	}
	logText := string(logData)
	for _, want := range []string{"sync-counts --apply", "promote --workspace " + workspace} {
		if !strings.Contains(logText, want) {
			t.Fatalf("helper log missing %q in %q", want, logText)
		}
	}
}

func TestPostEditHandleAndRunPostEdit(t *testing.T) {
	oldHome := os.Getenv("HOME")
	home := t.TempDir()
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatal(err)
	}
	defer os.Setenv("HOME", oldHome)

	p := config.DefaultPaths()
	if err := os.MkdirAll(p.HooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	h := &postEditHandler{
		log:   logger.New(filepath.Join(home, "post-edit.log")),
		paths: p,
	}

	filePath := filepath.Join(home, "note.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	resp, err := h.Handle(context.Background(), &hookio.Input{FilePath: filePath})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if resp.Permission != "" {
		t.Fatalf("Handle() permission = %q, want empty", resp.Permission)
	}
	events, err := metrics.Load(p.MetricsFile())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Hook != "post-edit" || events[0].Detail != "note.txt" {
		t.Fatalf("unexpected metrics events: %+v", events)
	}

	inFile := filepath.Join(home, "hook-input.json")
	outFile := filepath.Join(home, "hook-output.json")
	if err := os.WriteFile(inFile, []byte(`{"file_path":"`+filePath+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	in, err := os.Open(inFile)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.Create(outFile)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()

	if err := runPostEdit(in, out); err != nil {
		t.Fatalf("runPostEdit() error = %v", err)
	}
	outData, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(outData), "{}") {
		t.Fatalf("runPostEdit() output = %q, want empty hook response", string(outData))
	}
}
