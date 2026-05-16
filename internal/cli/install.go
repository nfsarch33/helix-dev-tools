package cli

import (
	"embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"

	"github.com/nfsarch33/cursor-tools/manifest"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"
)

// installManifest is the top-level structure of the embedded manifest.
type installManifest struct {
	Components map[string]installComponent `yaml:"components"`
}

// installComponent describes a single installable unit.
type installComponent struct {
	Description   string                        `yaml:"description"`
	Binary        string                        `yaml:"binary,omitempty"`
	Source        string                        `yaml:"source,omitempty"`
	Target        string                        `yaml:"target,omitempty"`
	Check         string                        `yaml:"check,omitempty"`
	Platforms     []string                      `yaml:"platforms"`
	Subcomponents map[string]installSubcomponent `yaml:"subcomponents,omitempty"`
}

// installSubcomponent describes a daemon sub-unit with platform-specific
// service managers.
type installSubcomponent struct {
	Launchd string `yaml:"launchd,omitempty"`
	Systemd string `yaml:"systemd,omitempty"`
}

type installResult struct {
	Name   string
	Status string // OK, SKIP, FAIL, NEED, LIST
	Detail string
}

var (
	installAll   bool
	installCheck bool
	installList  bool
)

var installCmd = &cobra.Command{
	Use:   "install [component...]",
	Short: "Install Helixon platform components from manifest",
	Long: `Manifest-driven installer for platform components.

  cursor-tools install runx          install a single component
  cursor-tools install --all         install all components for this platform
  cursor-tools install --check       dry-run — report what needs installing
  cursor-tools install --list        list available components`,
	RunE:         runInstall,
	SilenceUsage: true,
}

func init() {
	installCmd.Flags().BoolVar(&installAll, "all", false, "install all components for this platform")
	installCmd.Flags().BoolVar(&installCheck, "check", false, "dry-run: report what needs installing without making changes")
	installCmd.Flags().BoolVar(&installList, "list", false, "list available components and exit")
}

// loadManifest parses the embedded YAML manifest from the manifest package.
func loadManifest() (*installManifest, error) {
	return loadManifestFrom(manifest.FS)
}

// loadManifestFrom parses a manifest from an arbitrary embed.FS, enabling
// tests to inject fixture manifests.
func loadManifestFrom(fs embed.FS) (*installManifest, error) {
	data, err := fs.ReadFile("install.yaml")
	if err != nil {
		return nil, fmt.Errorf("read embedded manifest: %w", err)
	}
	var m installManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	return &m, nil
}

// expandEnv replaces $HOME and other env vars in a path string.
func expandEnv(s string) string {
	return os.ExpandEnv(s)
}

// platformMatch returns true when the component declares the given
// platform in its platforms list.
func platformMatch(platforms []string, goos string) bool {
	for _, p := range platforms {
		if p == goos {
			return true
		}
	}
	return false
}

// sortedComponentNames returns deterministic ordering for iteration.
func sortedComponentNames(m *installManifest) []string {
	names := make([]string, 0, len(m.Components))
	for k := range m.Components {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// checkInstalled runs the component's check command and returns true when
// the command exits 0.
func checkInstalled(checkExpr string, runner commandRunner) bool {
	if checkExpr == "" {
		return false
	}
	expanded := expandEnv(checkExpr)
	parts := strings.Fields(expanded)
	if len(parts) == 0 {
		return false
	}
	if runner != nil {
		return runner(parts[0], parts[1:]...) == nil
	}
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}

// commandRunner is an optional function used for dependency injection in tests.
type commandRunner func(name string, args ...string) error

// installOpts bundles the flags and DI seams for runInstallWith.
type installOpts struct {
	out    io.Writer
	args   []string
	all    bool
	check  bool
	list   bool
	runner commandRunner
	goos   string // override runtime.GOOS for testing
}

// runInstallWith is the core implementation, accepting options for testability.
func runInstallWith(opts installOpts) ([]installResult, error) {
	m, err := loadManifest()
	if err != nil {
		return nil, err
	}

	goos := opts.goos
	if goos == "" {
		goos = runtime.GOOS
	}

	if opts.list {
		return printComponentList(opts.out, m, goos), nil
	}

	targets, err := resolveTargets(m, opts.args, opts.all)
	if err != nil {
		return nil, err
	}

	var results []installResult
	for _, name := range targets {
		comp := m.Components[name]
		if !platformMatch(comp.Platforms, goos) {
			r := installResult{Name: name, Status: "SKIP", Detail: fmt.Sprintf("platform %s not in %v", goos, comp.Platforms)}
			results = append(results, r)
			fmt.Fprintf(opts.out, "  [SKIP] %-20s %s\n", name, r.Detail)
			continue
		}

		installed := checkInstalled(comp.Check, opts.runner)
		if installed {
			r := installResult{Name: name, Status: "OK", Detail: "already installed"}
			results = append(results, r)
			fmt.Fprintf(opts.out, "  [OK]   %-20s %s\n", name, r.Detail)
			continue
		}

		if opts.check {
			r := installResult{Name: name, Status: "NEED", Detail: "not installed (dry-run)"}
			results = append(results, r)
			fmt.Fprintf(opts.out, "  [NEED] %-20s %s\n", name, r.Detail)
			continue
		}

		r := doInstall(name, comp, opts.runner)
		results = append(results, r)
		fmt.Fprintf(opts.out, "  [%s] %-20s %s\n", r.Status, name, r.Detail)
	}

	return results, nil
}

// doInstall performs the actual installation for a single component.
func doInstall(name string, comp installComponent, runner commandRunner) installResult {
	if comp.Source != "" {
		expanded := expandEnv(comp.Source)
		parts := strings.Fields(expanded)
		if len(parts) == 0 {
			return installResult{Name: name, Status: "FAIL", Detail: "empty source command"}
		}
		var err error
		if runner != nil {
			err = runner(parts[0], parts[1:]...)
		} else {
			cmd := exec.Command(parts[0], parts[1:]...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err = cmd.Run()
		}
		if err != nil {
			return installResult{Name: name, Status: "FAIL", Detail: fmt.Sprintf("build failed: %v", err)}
		}
		return installResult{Name: name, Status: "OK", Detail: "installed"}
	}

	if comp.Binary != "" {
		path := expandEnv(comp.Binary)
		if _, err := os.Stat(path); err != nil {
			return installResult{Name: name, Status: "FAIL", Detail: fmt.Sprintf("binary not found at %s", path)}
		}
		return installResult{Name: name, Status: "OK", Detail: "binary present"}
	}

	if comp.Target != "" {
		path := expandEnv(comp.Target)
		if _, err := os.Stat(path); err != nil {
			return installResult{Name: name, Status: "FAIL", Detail: fmt.Sprintf("target not found at %s", path)}
		}
		return installResult{Name: name, Status: "OK", Detail: "target present"}
	}

	return installResult{Name: name, Status: "FAIL", Detail: "no install method defined"}
}

func printComponentList(out io.Writer, m *installManifest, goos string) []installResult {
	names := sortedComponentNames(m)
	var results []installResult
	for _, name := range names {
		comp := m.Components[name]
		supported := platformMatch(comp.Platforms, goos)
		marker := " "
		if !supported {
			marker = "~"
		}
		fmt.Fprintf(out, "  %s %-20s %s  [%s]\n", marker, name, comp.Description, strings.Join(comp.Platforms, ", "))
		results = append(results, installResult{Name: name, Status: "LIST"})
	}
	return results
}

func resolveTargets(m *installManifest, args []string, all bool) ([]string, error) {
	if all {
		return sortedComponentNames(m), nil
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("specify a component name or use --all; see --list for options")
	}
	for _, name := range args {
		if _, ok := m.Components[name]; !ok {
			available := sortedComponentNames(m)
			return nil, fmt.Errorf("unknown component %q; available: %s", name, strings.Join(available, ", "))
		}
	}
	return args, nil
}

func runInstall(cmd *cobra.Command, args []string) error {
	results, err := runInstallWith(installOpts{
		out:   cmd.OutOrStdout(),
		args:  args,
		all:   installAll,
		check: installCheck,
		list:  installList,
	})
	if err != nil {
		return err
	}
	for _, r := range results {
		if r.Status == "FAIL" {
			return fmt.Errorf("one or more components failed to install")
		}
	}
	return nil
}
