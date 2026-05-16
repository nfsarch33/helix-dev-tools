package installer

// DaemonConfig describes how a daemon should be installed on the host
// init system (launchd on macOS, systemd on Linux).
type DaemonConfig struct {
	Label       string
	Binary      string
	Args        []string
	WorkingDir  string
	LogPath     string
	Interval    int // seconds between invocations for periodic daemons (0 = long-running)
	Environment map[string]string
}

// DaemonStatus is the runtime state of an installed daemon.
type DaemonStatus struct {
	Installed bool
	Running   bool
	PID       int
	LastExit  int
}

// Installer is the platform-agnostic interface both LaunchdInstaller
// and SystemdInstaller implement.
type Installer interface {
	Install(name string, config DaemonConfig) error
	Uninstall(name string) error
	Status(name string) (DaemonStatus, error)
	IsInstalled(name string) bool
}

// CommandExecutor abstracts os/exec for testability.
type CommandExecutor interface {
	Run(name string, args ...string) ([]byte, error)
}
