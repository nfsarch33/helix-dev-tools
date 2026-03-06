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

func runSafe(_ *cobra.Command, _ []string) error {
	var cursorPath string
	switch runtime.GOOS {
	case "darwin":
		cursorPath = "/Applications/Cursor.app/Contents/MacOS/Cursor"
	default:
		cursorPath = "cursor"
	}

	cmd := exec.Command(cursorPath, "--disable-gpu")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Start()
}
