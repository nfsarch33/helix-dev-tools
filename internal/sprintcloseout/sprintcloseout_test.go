package sprintcloseout_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/sprintcloseout"
)

func TestEvidence_RequiredFilesPresent(t *testing.T) {
	dir := t.TempDir()
	id := "v6074"

	// Create the 7 required evidence files.
	required := sprintcloseout.RequiredFiles(id)
	for _, name := range required {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("evidence"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	result := sprintcloseout.Check(id, dir)
	if !result.OK {
		t.Errorf("Check() not OK: missing=%v", result.Missing)
	}
	if len(result.Missing) != 0 {
		t.Errorf("expected 0 missing, got %v", result.Missing)
	}
}

func TestEvidence_MissingFiles(t *testing.T) {
	dir := t.TempDir()
	id := "v6074"

	// Only write half the required files.
	required := sprintcloseout.RequiredFiles(id)
	for i, name := range required {
		if i%2 == 0 {
			continue // skip half
		}
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("evidence"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	result := sprintcloseout.Check(id, dir)
	if result.OK {
		t.Errorf("Check() returned OK but half files are missing")
	}
	if len(result.Missing) == 0 {
		t.Errorf("expected missing files, got none")
	}
}

func TestEvidence_Report(t *testing.T) {
	dir := t.TempDir()
	id := "v9999"

	result := sprintcloseout.Check(id, dir)
	report := result.Report()

	if !strings.Contains(report, id) {
		t.Errorf("Report() missing sprint ID %q: %s", id, report)
	}
	if !strings.Contains(report, "INCOMPLETE") && !strings.Contains(report, "COMPLETE") {
		t.Errorf("Report() missing status: %s", report)
	}
}

func TestEvidence_AllRequiredFileNames(t *testing.T) {
	// Sprint closeout requires exactly 7 artefact types (per sprint-scaffold rule).
	files := sprintcloseout.RequiredFiles("v6074")
	if len(files) != 7 {
		t.Errorf("RequiredFiles() = %d files, want 7", len(files))
	}

	expected := []string{"retro", "kpi", "capsule", "handoff", "evidence", "badge", "evospine"}
	for _, keyword := range expected {
		found := false
		for _, f := range files {
			if strings.Contains(strings.ToLower(f), keyword) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("RequiredFiles() missing file containing %q", keyword)
		}
	}
}

func TestRebuildSpec_BuildCommand(t *testing.T) {
	spec := sprintcloseout.RebuildSpec{
		RepoPath: "/tmp/cursor-tools",
		MakeTarget: "install",
	}
	cmd := spec.BuildCommand()
	if !strings.Contains(cmd, "make") {
		t.Errorf("BuildCommand() %q missing 'make'", cmd)
	}
	if !strings.Contains(cmd, spec.MakeTarget) {
		t.Errorf("BuildCommand() %q missing target %q", cmd, spec.MakeTarget)
	}
}

func TestRebuildSpec_Validate(t *testing.T) {
	tests := []struct {
		name    string
		spec    sprintcloseout.RebuildSpec
		wantErr bool
	}{
		{"valid", sprintcloseout.RebuildSpec{RepoPath: "/tmp/r", MakeTarget: "install"}, false},
		{"empty repo", sprintcloseout.RebuildSpec{MakeTarget: "install"}, true},
		{"empty target", sprintcloseout.RebuildSpec{RepoPath: "/tmp/r"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.spec.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
