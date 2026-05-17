package ansiblevalidator

// SSHRoute represents a single SSH path to a target.
type SSHRoute struct {
	Alias       string
	Description string
	ProxyJump   string
	Port        int
	Fallback    *SSHRoute
}

// SSHMatrix holds all known SSH routes to fleet hosts.
type SSHMatrix struct {
	Routes map[string]SSHRoute
}

// SSHMatrixProbeResult holds the outcome of probing an entire SSH matrix.
type SSHMatrixProbeResult struct {
	Results   map[string]SSHCanaryResult
	Reachable int
	Total     int
}

// WslExeFallbackResult holds the outcome of a wsl.exe relay probe.
type WslExeFallbackResult struct {
	HostAlias  string
	WslDistro  string
	Reachable  bool
	Output     string
	Error      string
}
