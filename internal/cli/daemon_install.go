package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/nfsarch33/helix-dev-tools/internal/installer"
	"github.com/spf13/cobra"
)

// knownDaemons maps short names to their DaemonConfig. The manifest's
// subcomponents reference these by name; this is the runtime registry
// that backs `cursor-tools daemon install <name>`.
var knownDaemons = map[string]installer.DaemonConfig{
	"cursor-resource-probe": {
		Label:    "com.user.cursor-resource-probe",
		Binary:   binaryPath(),
		Args:     []string{"resource-probe", "--interval", "300"},
		LogPath:  logPath("cursor-resource-probe"),
		Interval: 300,
	},
	"cursor-fleet-health": {
		Label:   "com.user.cursor-fleet-health",
		Binary:  binaryPath(),
		Args:    []string{"health-check"},
		LogPath: logPath("cursor-fleet-health"),
	},
}

func binaryPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "cursor-tools"
	}
	return exe
}

func logPath(name string) string {
	home, _ := os.UserHomeDir()
	return home + "/.local/log/" + name + ".log"
}

// platformLabel returns the platform-appropriate service label.
func platformLabel(name string) string {
	if runtime.GOOS == "darwin" {
		return "com.user." + name
	}
	return name + ".service"
}

var daemonInstallCmd = &cobra.Command{
	Use:   "install <name>",
	Short: "Install a known daemon into the platform init system",
	Long: `Installs a daemon as a launchd LaunchAgent (macOS) or systemd user unit (Linux).

Known daemons:
  cursor-resource-probe    5-minute memory pressure probe
  cursor-fleet-health      fleet health daemon`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDaemonInstall(cmd.OutOrStdout(), args[0], runtime.GOOS)
	},
}

var daemonUninstallCmd = &cobra.Command{
	Use:   "uninstall <name>",
	Short: "Remove a daemon from the platform init system",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDaemonUninstall(cmd.OutOrStdout(), args[0], runtime.GOOS)
	},
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status [name]",
	Short: "Show status of installed daemons",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := ""
		if len(args) > 0 {
			name = args[0]
		}
		return runDaemonStatus(cmd.OutOrStdout(), name, runtime.GOOS)
	},
}

func init() {
	daemonCmd.AddCommand(daemonInstallCmd)
	daemonCmd.AddCommand(daemonUninstallCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
}

// --- testable implementations ---

// realExec implements installer.CommandExecutor using os/exec.
type realExec struct{}

func (r *realExec) Run(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

// newInstaller returns the platform-appropriate Installer.
var newInstallerFunc = newInstallerDefault

func newInstallerDefault(goos string) (installer.Installer, error) {
	exec := &realExec{}
	switch goos {
	case "darwin":
		return installer.NewLaunchdInstaller(exec), nil
	case "linux":
		return installer.NewSystemdInstaller(exec), nil
	default:
		return nil, fmt.Errorf("unsupported platform: %s", goos)
	}
}

func runDaemonInstall(out io.Writer, name, goos string) error {
	cfg, ok := knownDaemons[name]
	if !ok {
		return fmt.Errorf("unknown daemon %q; known: %s", name, knownDaemonNames())
	}

	if goos == "linux" {
		cfg.Label = name
	}

	inst, err := newInstallerFunc(goos)
	if err != nil {
		return err
	}

	label := platformLabel(name)
	if err := inst.Install(label, cfg); err != nil {
		return fmt.Errorf("install %s: %w", name, err)
	}

	fmt.Fprintf(out, "  [OK] %s installed as %s\n", name, label)
	return nil
}

func runDaemonUninstall(out io.Writer, name, goos string) error {
	if _, ok := knownDaemons[name]; !ok {
		return fmt.Errorf("unknown daemon %q; known: %s", name, knownDaemonNames())
	}

	inst, err := newInstallerFunc(goos)
	if err != nil {
		return err
	}

	label := platformLabel(name)
	if err := inst.Uninstall(label); err != nil {
		return fmt.Errorf("uninstall %s: %w", name, err)
	}

	fmt.Fprintf(out, "  [OK] %s uninstalled\n", name)
	return nil
}

func runDaemonStatus(out io.Writer, name, goos string) error {
	inst, err := newInstallerFunc(goos)
	if err != nil {
		return err
	}

	targets := make(map[string]installer.DaemonConfig)
	if name != "" {
		cfg, ok := knownDaemons[name]
		if !ok {
			return fmt.Errorf("unknown daemon %q; known: %s", name, knownDaemonNames())
		}
		targets[name] = cfg
	} else {
		for k, v := range knownDaemons {
			targets[k] = v
		}
	}

	for n := range targets {
		label := platformLabel(n)
		st, err := inst.Status(label)
		if err != nil {
			fmt.Fprintf(out, "  [ERR]  %-30s %v\n", n, err)
			continue
		}

		state := "not installed"
		if st.Installed && st.Running {
			state = fmt.Sprintf("running (pid=%d)", st.PID)
		} else if st.Installed {
			state = fmt.Sprintf("stopped (last_exit=%d)", st.LastExit)
		}

		fmt.Fprintf(out, "  %-30s %s\n", n, state)
	}

	return nil
}

func knownDaemonNames() string {
	names := make([]string, 0, len(knownDaemons))
	for k := range knownDaemons {
		names = append(names, k)
	}
	return strings.Join(names, ", ")
}
