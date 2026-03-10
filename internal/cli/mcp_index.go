package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/cursor-tools/internal/clilog"
	"github.com/nfsarch33/cursor-tools/internal/config"
)

var mcpIndexFlags struct {
	mcpJSON string
	out     string
}

var mcpIndexCmd = &cobra.Command{
	Use:   "mcp-index",
	Short: "Refresh Pepper MCP index from local ~/.cursor/mcp.json (values redacted)",
	Long:  "Reads the Cursor MCP config, redacts env values, and writes a Markdown index to Pepper.",
	RunE:  runMCPIndex,
}

func init() {
	p := config.DefaultPaths()
	mcpIndexCmd.Flags().StringVar(
		&mcpIndexFlags.mcpJSON,
		"mcp-json",
		filepath.Join(p.Home, ".cursor", "mcp.json"),
		"Path to Cursor MCP config",
	)
	mcpIndexCmd.Flags().StringVar(
		&mcpIndexFlags.out,
		"out",
		filepath.Join(p.GlobalMemoriesDir(), "mcp-index-and-selection-sop.md"),
		"Output Markdown file in Pepper",
	)
}

// mcpServerSpec mirrors the relevant fields of an MCP server entry.
type mcpServerSpec struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
	Type    string            `json:"type"`
	URL     string            `json:"url"`
}

type mcpConfig struct {
	MCPServers map[string]mcpServerSpec `json:"mcpServers"`
}

// loadMCPServers reads and parses the MCP config file.
func loadMCPServers(path string) (map[string]mcpServerSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading mcp.json: %w", err)
	}
	var cfg mcpConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing mcp.json: %w", err)
	}
	if cfg.MCPServers == nil {
		cfg.MCPServers = make(map[string]mcpServerSpec)
	}
	return cfg.MCPServers, nil
}

// credentialFlags lists CLI flag names whose values should be redacted.
var credentialFlags = []string{
	"--token", "--jira-token", "--api-key", "--api-token",
	"--secret", "--password", "--credentials",
}

// redactArgs returns a copy of args with credential values replaced by "***REDACTED***".
func redactArgs(args []string) []string {
	out := make([]string, len(args))
	copy(out, args)
	for i := 0; i < len(out); i++ {
		for _, flag := range credentialFlags {
			if out[i] == flag && i+1 < len(out) {
				out[i+1] = "***REDACTED***"
				break
			}
			if strings.HasPrefix(out[i], flag+"=") {
				out[i] = flag + "=***REDACTED***"
				break
			}
		}
	}
	return out
}

// renderMCPIndex generates the Markdown index content with env values and credential args redacted.
func renderMCPIndex(servers map[string]mcpServerSpec) string {
	now := time.Now().Format(time.RFC3339)

	var b strings.Builder
	b.WriteString("# MCP index + tool selection SOP (Cursor)\n\n")
	b.WriteString("Last generated: " + now + "\n")
	b.WriteString(fmt.Sprintf("Server count: %d\n\n", len(servers)))

	b.WriteString("## Why this exists\n")
	b.WriteString("When you have hundreds of MCP tools, the goal is fast, safe selection with minimal context switching.\n")
	b.WriteString("This file is canonical in Pepper (git): `~/memo/global-memories/mcp-index-and-selection-sop.md`.\n\n")

	b.WriteString("## Tool selection SOP (KISS)\n")
	b.WriteString("- If the task is read-only codebase investigation: use `context-mode` first, then `ReadFile`, `rg`, and `Glob`.\n")
	b.WriteString("- If the task mutates files or git state: use the normal edit tools for files and Shell for installs, builds, and git commands.\n")
	b.WriteString("- If the task is Git operations: prefer `git-mcp-server` for repo inspection and Shell/`gh` for workflow actions that need full git semantics.\n")
	b.WriteString("- If the task is live research or current docs: prefer `perplexity`, `context7`, and `fetch` before generic web browsing.\n")
	b.WriteString("- If the task is docs/word: use `word-document-server`.\n")
	b.WriteString("- If the task is PDF ops: use `pdf-handler` (form fill/clear, comments, text, signatures, encrypt).\n")
	b.WriteString("- If the task is memory/rules: Pepper is canonical; internal memory only stores pointers + short invariants.\n\n")

	b.WriteString("## Quality gates (non-breaking)\n")
	b.WriteString("- Default to non-breaking changes. Ask before breaking changes.\n")
	b.WriteString("- Before relying on a fresh install or resumed machine state: run `~/bin/cursor-tools doctor mcp`, `doctor platform`, and `selftest`.\n\n")

	b.WriteString("## Config hygiene\n")
	b.WriteString("- Local dev may keep static creds in `~/.cursor/mcp.json` if needed.\n")
	b.WriteString("- Never commit secrets into repos or rule files.\n\n")

	b.WriteString("## Available MCP servers (redacted)\n\n")

	names := make([]string, 0, len(servers))
	for name := range servers {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		spec := servers[name]
		b.WriteString("### " + name + "\n")
		b.WriteString("- command: `" + spec.Command + "`\n")
		safeArgs := redactArgs(spec.Args)
		b.WriteString(fmt.Sprintf("- args: `%v`\n", safeArgs))
		if len(spec.Env) > 0 {
			envKeys := make([]string, 0, len(spec.Env))
			for k := range spec.Env {
				envKeys = append(envKeys, k)
			}
			sort.Strings(envKeys)
			b.WriteString(fmt.Sprintf("- env keys: `%v` (values redacted)\n", envKeys))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// refreshMCPIndex is the core logic shared by the CLI command and daily-refresh step.
// Returns true if the file was updated, false if unchanged.
func refreshMCPIndex(mcpJSONPath, outPath string) (bool, error) {
	servers, err := loadMCPServers(mcpJSONPath)
	if err != nil {
		return false, err
	}

	rendered := renderMCPIndex(servers)

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return false, fmt.Errorf("creating output directory: %w", err)
	}

	current, err := os.ReadFile(outPath)
	if err == nil {
		// Compare ignoring the "Last generated:" timestamp line to avoid
		// needless rewrites when nothing else changed.
		if stripTimestamp(string(current)) == stripTimestamp(rendered) {
			return false, nil
		}
	}

	if err := os.WriteFile(outPath, []byte(rendered), 0o644); err != nil {
		return false, fmt.Errorf("writing index: %w", err)
	}
	return true, nil
}

// stripTimestamp removes the "Last generated: ..." line for comparison purposes.
func stripTimestamp(s string) string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		if !strings.HasPrefix(line, "Last generated:") {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

func runMCPIndex(_ *cobra.Command, _ []string) error {
	updated, err := refreshMCPIndex(mcpIndexFlags.mcpJSON, mcpIndexFlags.out)
	if err != nil {
		return err
	}
	if updated {
		clilog.Success("updated %s", mcpIndexFlags.out)
	} else {
		clilog.Success("no changes (%s)", mcpIndexFlags.out)
	}
	return nil
}
