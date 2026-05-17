package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/helix-dev-tools/internal/clilog"
	"github.com/nfsarch33/helix-dev-tools/internal/config"
)

var autoUpdateCheck bool
var autoUpdateForce bool

var autoUpdateCmd = &cobra.Command{
	Use:   "auto-update",
	Short: "Install the latest pre-built cursor-tools binary from dist/",
	RunE: func(_ *cobra.Command, _ []string) error {
		return applyDistUpdate(config.DefaultPaths(), clilog.NewPrefixed("[auto-update]"), autoUpdateCheck, autoUpdateForce)
	},
}

func init() {
	autoUpdateCmd.Flags().BoolVar(&autoUpdateCheck, "check", false, "Check whether a dist update is available without applying it")
	autoUpdateCmd.Flags().BoolVar(&autoUpdateForce, "force", false, "Install the dist binary even if it is not newer")
}

func applyDistUpdate(p config.Paths, out *clilog.Prefixed, checkOnly, force bool) error {
	distDir := filepath.Join(p.CursorConfigDir(), "cursor-tools", "dist")
	distVersionPath := filepath.Join(distDir, "VERSION")
	distVersion := strings.TrimSpace(readOptionalFile(distVersionPath))

	platform := p.PlatformBinarySuffix()
	distBinary := filepath.Join(distDir, "cursor-tools-"+platform)
	localBinary := filepath.Join(p.BinDir, "cursor-tools")

	distInfo, err := os.Stat(distBinary)
	if err != nil {
		if checkOnly {
			out.Info("no dist binary available for %s", platform)
		}
		return nil
	}

	localInfo, err := os.Stat(localBinary)
	hasLocalBinary := err == nil
	needsUpdate := force || !hasLocalBinary || distInfo.ModTime().After(localInfo.ModTime())

	versionLabel := platform
	if distVersion != "" {
		versionLabel = distVersion + " (" + platform + ")"
	}

	if !needsUpdate {
		if checkOnly {
			out.Info("up to date with dist: %s", versionLabel)
		}
		return nil
	}

	if checkOnly {
		out.Info("update available from dist: %s", versionLabel)
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(localBinary), 0o755); err != nil {
		return fmt.Errorf("create bin dir: %w", err)
	}
	if err := installBinary(distBinary, localBinary); err != nil {
		return fmt.Errorf("install dist binary: %w", err)
	}

	out.Info("installed cursor-tools from dist: %s", versionLabel)
	return nil
}

func installBinary(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(dst), "cursor-tools-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, info.Mode()); err != nil {
		return err
	}
	return os.Rename(tmpPath, dst)
}

func readOptionalFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}
