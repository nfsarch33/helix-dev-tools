package devex

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScaffolder_Generate(t *testing.T) {
	s := NewScaffolder()
	s.Register(Template{
		Name: "go-package",
		Files: map[string]string{
			"{{name}}.go":      "package {{name}}\n",
			"{{name}}_test.go": "package {{name}}\n\nimport \"testing\"\n",
		},
	})

	dir := t.TempDir()
	err := s.Generate("go-package", dir, map[string]string{"name": "mylib"})
	if err != nil {
		t.Fatalf("generate error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "mylib.go"))
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if string(content) != "package mylib\n" {
		t.Errorf("unexpected content: %s", content)
	}
}

func TestScaffolder_TemplateNotFound(t *testing.T) {
	s := NewScaffolder()
	err := s.Generate("missing", t.TempDir(), nil)
	if err == nil {
		t.Error("expected error for missing template")
	}
}

func TestScaffolder_List(t *testing.T) {
	s := NewScaffolder()
	s.Register(Template{Name: "a"})
	s.Register(Template{Name: "b"})

	if len(s.List()) != 2 {
		t.Errorf("expected 2 templates")
	}
}

func TestScaffolder_FileCount(t *testing.T) {
	s := NewScaffolder()
	s.Register(Template{Name: "t", Files: map[string]string{"a": "", "b": "", "c": ""}})

	if s.FileCount("t") != 3 {
		t.Errorf("expected 3 files")
	}
	if s.FileCount("missing") != 0 {
		t.Error("expected 0 for missing template")
	}
}

func TestScaffolder_NestedDirs(t *testing.T) {
	s := NewScaffolder()
	s.Register(Template{Name: "nested", Files: map[string]string{"sub/dir/file.go": "content"}})

	dir := t.TempDir()
	err := s.Generate("nested", dir, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = os.Stat(filepath.Join(dir, "sub", "dir", "file.go"))
	if err != nil {
		t.Error("expected nested file to exist")
	}
}
