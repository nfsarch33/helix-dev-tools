package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nfsarch33/cursor-tools/internal/clilog"
	"github.com/nfsarch33/cursor-tools/internal/config"
)

func TestApplyDistUpdateInstallsNewerBinary(t *testing.T) {
	home := t.TempDir()
	globalKB := filepath.Join(home, "Code", "global-kb")
	distDir := filepath.Join(globalKB, "cursor-config", "cursor-tools", "dist")
	binDir := filepath.Join(home, "bin")

	if err := os.MkdirAll(distDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}

	p := config.Paths{
		Home:     home,
		GlobalKB: globalKB,
		Memo:     filepath.Join(home, "memo"),
		BinDir:   binDir,
	}

	localBinary := filepath.Join(binDir, "cursor-tools")
	distBinary := filepath.Join(distDir, "cursor-tools-"+p.PlatformBinarySuffix())
	versionFile := filepath.Join(distDir, "VERSION")

	if err := os.WriteFile(localBinary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(distBinary, []byte("new-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(versionFile, []byte("v1.2.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := applyDistUpdate(p, clilog.NewPrefixed("[test]"), false, false); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(localBinary)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new-binary" {
		t.Fatalf("local binary = %q, want dist contents", string(got))
	}
}

func TestApplyDistUpdateCheckOnlyLeavesBinaryUntouched(t *testing.T) {
	home := t.TempDir()
	globalKB := filepath.Join(home, "Code", "global-kb")
	distDir := filepath.Join(globalKB, "cursor-config", "cursor-tools", "dist")
	binDir := filepath.Join(home, "bin")

	if err := os.MkdirAll(distDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}

	p := config.Paths{
		Home:     home,
		GlobalKB: globalKB,
		Memo:     filepath.Join(home, "memo"),
		BinDir:   binDir,
	}

	localBinary := filepath.Join(binDir, "cursor-tools")
	distBinary := filepath.Join(distDir, "cursor-tools-"+p.PlatformBinarySuffix())

	if err := os.WriteFile(localBinary, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(distBinary, []byte("new-binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := applyDistUpdate(p, clilog.NewPrefixed("[test]"), true, false); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(localBinary)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old-binary" {
		t.Fatalf("local binary changed during check-only mode: %q", string(got))
	}
}
