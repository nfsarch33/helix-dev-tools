package cli

import (
	"bytes"
	"fmt"
	"runtime"
	"strings"
	"testing"
)

func TestLoadManifest(t *testing.T) {
	m, err := loadManifest()
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}
	if len(m.Components) == 0 {
		t.Fatal("manifest has zero components")
	}
	want := []string{"runx", "sentrux", "mem0-oss-client", "hooks", "daemons"}
	for _, name := range want {
		if _, ok := m.Components[name]; !ok {
			t.Errorf("missing expected component %q", name)
		}
	}
}

func TestManifestComponentFields(t *testing.T) {
	m, err := loadManifest()
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}

	runx := m.Components["runx"]
	if runx.Description == "" {
		t.Error("runx.Description is empty")
	}
	if runx.Binary != "$HOME/runs/runx" {
		t.Errorf("runx.Binary = %q, want $HOME/runs/runx", runx.Binary)
	}
	if runx.Check == "" {
		t.Error("runx.Check is empty")
	}
	if len(runx.Platforms) == 0 {
		t.Error("runx.Platforms is empty")
	}
}

func TestManifestSubcomponents(t *testing.T) {
	m, err := loadManifest()
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}
	daemons := m.Components["daemons"]
	if len(daemons.Subcomponents) == 0 {
		t.Fatal("daemons.Subcomponents is empty")
	}
	rp, ok := daemons.Subcomponents["resource-probe"]
	if !ok {
		t.Fatal("missing subcomponent resource-probe")
	}
	if rp.Launchd != "com.user.cursor-resource-probe" {
		t.Errorf("resource-probe.Launchd = %q", rp.Launchd)
	}
	if rp.Systemd != "cursor-resource-probe.service" {
		t.Errorf("resource-probe.Systemd = %q", rp.Systemd)
	}
}

func TestPlatformMatch(t *testing.T) {
	tests := []struct {
		platforms []string
		goos      string
		want      bool
	}{
		{[]string{"darwin", "linux"}, "darwin", true},
		{[]string{"darwin", "linux"}, "linux", true},
		{[]string{"darwin", "linux"}, "windows", false},
		{[]string{"linux"}, "darwin", false},
		{nil, "darwin", false},
	}
	for _, tt := range tests {
		got := platformMatch(tt.platforms, tt.goos)
		if got != tt.want {
			t.Errorf("platformMatch(%v, %q) = %v, want %v", tt.platforms, tt.goos, got, tt.want)
		}
	}
}

func TestSortedComponentNames(t *testing.T) {
	m, err := loadManifest()
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}
	names := sortedComponentNames(m)
	if len(names) != len(m.Components) {
		t.Fatalf("got %d names, want %d", len(names), len(m.Components))
	}
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Errorf("names not sorted: %q before %q", names[i-1], names[i])
		}
	}
}

func TestResolveTargets_All(t *testing.T) {
	m, err := loadManifest()
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}
	targets, err := resolveTargets(m, nil, true)
	if err != nil {
		t.Fatalf("resolveTargets --all: %v", err)
	}
	if len(targets) != len(m.Components) {
		t.Errorf("got %d targets, want %d", len(targets), len(m.Components))
	}
}

func TestResolveTargets_Named(t *testing.T) {
	m, err := loadManifest()
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}
	targets, err := resolveTargets(m, []string{"runx"}, false)
	if err != nil {
		t.Fatalf("resolveTargets runx: %v", err)
	}
	if len(targets) != 1 || targets[0] != "runx" {
		t.Errorf("got %v, want [runx]", targets)
	}
}

func TestResolveTargets_Unknown(t *testing.T) {
	m, err := loadManifest()
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}
	_, err = resolveTargets(m, []string{"nonexistent"}, false)
	if err == nil {
		t.Fatal("expected error for unknown component")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error %q does not mention nonexistent", err)
	}
}

func TestResolveTargets_NoArgs(t *testing.T) {
	m, err := loadManifest()
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}
	_, err = resolveTargets(m, nil, false)
	if err == nil {
		t.Fatal("expected error with no args and no --all")
	}
}

func TestInstallList(t *testing.T) {
	var buf bytes.Buffer
	results, err := runInstallWith(installOpts{
		out:  &buf,
		list: true,
		goos: "darwin",
	})
	if err != nil {
		t.Fatalf("runInstallWith --list: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected non-empty results")
	}
	for _, r := range results {
		if r.Status != "LIST" {
			t.Errorf("unexpected status %q for %s in --list mode", r.Status, r.Name)
		}
	}
	output := buf.String()
	if !strings.Contains(output, "runx") {
		t.Error("--list output missing runx")
	}
	if !strings.Contains(output, "sentrux") {
		t.Error("--list output missing sentrux")
	}
}

func TestInstallCheckDryRun(t *testing.T) {
	alwaysMissing := func(name string, args ...string) error {
		return fmt.Errorf("not found")
	}
	var buf bytes.Buffer
	results, err := runInstallWith(installOpts{
		out:    &buf,
		args:   []string{"runx"},
		check:  true,
		runner: alwaysMissing,
		goos:   runtime.GOOS,
	})
	if err != nil {
		t.Fatalf("runInstallWith --check: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "NEED" {
		t.Errorf("expected NEED, got %s", results[0].Status)
	}
}

func TestInstallCheckAlreadyInstalled(t *testing.T) {
	alwaysOK := func(name string, args ...string) error {
		return nil
	}
	var buf bytes.Buffer
	results, err := runInstallWith(installOpts{
		out:    &buf,
		args:   []string{"runx"},
		check:  true,
		runner: alwaysOK,
		goos:   runtime.GOOS,
	})
	if err != nil {
		t.Fatalf("runInstallWith: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "OK" {
		t.Errorf("expected OK, got %s", results[0].Status)
	}
}

func TestInstallPlatformSkip(t *testing.T) {
	var buf bytes.Buffer
	results, err := runInstallWith(installOpts{
		out:  &buf,
		all:  true,
		goos: "windows",
	})
	if err != nil {
		t.Fatalf("runInstallWith: %v", err)
	}
	for _, r := range results {
		if r.Status != "SKIP" {
			t.Errorf("component %s status = %s, want SKIP on windows", r.Name, r.Status)
		}
	}
}

func TestInstallAll_CheckMode(t *testing.T) {
	callLog := make(map[string]bool)
	runner := func(name string, args ...string) error {
		key := name + " " + strings.Join(args, " ")
		callLog[key] = true
		return fmt.Errorf("not installed")
	}
	var buf bytes.Buffer
	results, err := runInstallWith(installOpts{
		out:    &buf,
		all:    true,
		check:  true,
		runner: runner,
		goos:   "darwin",
	})
	if err != nil {
		t.Fatalf("runInstallWith: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results")
	}
	for _, r := range results {
		if r.Status != "NEED" {
			t.Errorf("component %s: status = %s, want NEED in check mode", r.Name, r.Status)
		}
	}
}

func TestCheckInstalled_EmptyExpr(t *testing.T) {
	if checkInstalled("", nil) {
		t.Error("empty check expression should return false")
	}
}

func TestCheckInstalled_WithRunner(t *testing.T) {
	ok := func(name string, args ...string) error { return nil }
	fail := func(name string, args ...string) error { return fmt.Errorf("fail") }

	if !checkInstalled("echo hello", ok) {
		t.Error("expected true for ok runner")
	}
	if checkInstalled("echo hello", fail) {
		t.Error("expected false for fail runner")
	}
}
