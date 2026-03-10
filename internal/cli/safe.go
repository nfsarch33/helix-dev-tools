package cli

import (
	"os"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
)

var safeCmd = &cobra.Command{
	Use:   "safe",
	Short: "Launch Cursor with --disable-gpu (workaround for macOS Electron bug)",
	RunE:  runSafe,
}

var safeCursorPath string // override in tests

func defaultCursorPath() string {
	if safeCursorPath != "" {
		return safeCursorPath
	}
	if runtime.GOOS == "darwin" {
		return "/Applications/Cursor.app/Contents/MacOS/Cursor"
	}
	return "cursor"
}

func runSafe(_ *cobra.Command, _ []string) error {
	cmd := exec.Command(defaultCursorPath(), "--disable-gpu")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Start()
}
