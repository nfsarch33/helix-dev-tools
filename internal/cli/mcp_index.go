package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
	check   bool
	write   bool
}

var mcpIndexCmd = &cobra.Command{
	Use:   "mcp-index",
	Short: "Refresh MCP index from local ~/.cursor/mcp.json",
	Long:  "Reads the Cursor MCP config, redacts env values, and writes a Markdown index for the Git-backed startup docs.",
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
	mcpIndexCmd.Flags().BoolVar(
		&mcpIndexFlags.check,
		"check",
		false,
		"Check whether the generated MCP index matches the output file without writing",
	)
	mcpIndexCmd.Flags().BoolVar(
		&mcpIndexFlags.write,
		"write",
		false,
		"Write the generated MCP index (default behaviour when --check is not set)",
	)
}

// mcpServerSpec mirrors the relevant fields of an MCP server entry.
type mcpServerSpec struct {
	Command  string            `json:"command"`
	Args     []string          `json:"args"`
	Env      map[string]string `json:"env"`
	Type     string            `json:"type"`
	URL      string            `json:"url"`
	Disabled bool              `json:"disabled"`
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

// urlQuerySecretRE masks likely API key or token values in query strings so generated indexes are safe to commit.
var urlQuerySecretRE = regexp.MustCompile(`([?&][^=&]*(?:[Kk]ey|[Tt]oken|[Ss]ecret|apikey)[^=&]*)=([^&]*)`)

func redactURLSecrets(raw string) string {
	if raw == "" {
		return raw
	}
	return urlQuerySecretRE.ReplaceAllString(raw, "${1}=***REDACTED***")
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
	b.WriteString("This file is canonical in the Git-backed startup docs: `~/memo/global-memories/mcp-index-and-selection-sop.md`.\n\n")

	b.WriteString("## Tool selection SOP (KISS)\n")
	b.WriteString("- If the task is read-only codebase investigation: use `context-mode` first, then `ReadFile`, `rg`, and `Glob`.\n")
	b.WriteString("- If the task mutates files or git state: use the normal edit tools for files and Shell for installs, builds, and git commands.\n")
	b.WriteString("- If the task is Git operations: prefer `git-mcp-server` for repo inspection and Shell/`gh` for workflow actions that need full git semantics.\n")
	b.WriteString("- If the task is live research or current docs: **tier order** — `perplexity_ask` / `perplexity_research` first; on **401, quota, or rate limit** → **`tavily`** / **`tavily-mcp`** MCP → **`exa`** MCP → then `duckduckgo`, `fetch`, `context7`, and `multi-search-engine` skill / built-in WebSearch before ad-hoc browsing.\n")
	b.WriteString("- If the task is docs/word: use `word-document-server`.\n")
	b.WriteString("- If the task is PDF ops: use `pdf-handler` (form fill/clear, comments, text, signatures, encrypt).\n")
	b.WriteString("- If the task is scholarly / peer-reviewed papers (titles, authors, abstracts, citations, year filters): use `google-scholar` (`search_google_scholar_key_words`, `search_google_scholar_advanced`, `get_author_info`) before falling back to `multi-search-engine` or web search.\n")
	b.WriteString("- If the task is job search / freelance cash flow: use `linkedin-mcp` for LinkedIn job, people, company, and light outreach research; use `upwork-mcp` for Upwork job, proposal, message, and contract workflows. Default to read-only discovery first. Never submit proposals, withdraw proposals, send messages, or connect with people without explicit human confirmation in the current turn.\n")
	b.WriteString("- If the task is shared memory, learnings, or recall across machines/agents: use `mem0` first (`search_memories`, `add_memory`, `update_memory`).\n")
	b.WriteString("- If the task is memory/rules: Mem0 is canonical for hot shared memory; Git-backed Pepper files remain the durable index/archive.\n\n")

	b.WriteString("## Local agent-path default\n")
	b.WriteString("- For local agent execution on this workstation, prefer the loopback-only IronClaw path first: `Cursor -> ironclaw-mcp -> IronClaw gateway -> llm-cluster-router -> local vLLM`.\n")
	b.WriteString("- Treat `ironclaw-mcp` as the canonical MCP bridge when a task needs the local secured runtime, tool execution, or local Qwen routing.\n")
	b.WriteString("- Before relying on the local path, verify `~/bin/cursor-tools doctor mcp`, `~/bin/cursor-tools health-check`, `~/bin/cursor-tools selftest`, and the `ironclaw-mcp` smoke harness.\n")
	b.WriteString("- Keep the local router as the only OpenAI-compatible endpoint IronClaw talks to. Do not bypass it from IronClaw directly to ad hoc vLLM ports.\n")
	b.WriteString("- Keep Gemini CLI as a secondary operator path, not the default local runtime path.\n\n")

	b.WriteString("## Freelancing / job board MCP safety\n")
	b.WriteString("- `linkedin-mcp` and `upwork-mcp` are personal, local-only browser-session MCPs. They are not fleet services and must not run on Tailscale, OCI, WSL member nodes, or shared hosts.\n")
	b.WriteString("- Session directories are account secrets: `~/.linkedin-mcp/profile` and `~/.upwork-mcp/chrome-profile`. Treat them like credentials; do not back them up into Git, Mem0, logs, or evidence bundles.\n")
	b.WriteString("- Use low-volume, read-mostly workflows to reduce platform ToS and account-risk exposure. Human approval is mandatory for write/social actions: LinkedIn `send_message` / `connect_with_person`, Upwork `upwork_submit_proposal` / `upwork_withdraw_proposal` / `upwork_send_message`.\n")
	b.WriteString("- `upwork-mcp` uses dedicated CDP port `19222` via `UPWORK_MCP_CDP_PORT` to avoid collisions with other Chrome debuggers on `9222`.\n")
	b.WriteString("- `upwork_get_my_profile` currently needs manual verification: the scraper can land on the settings page and return weak data, and `upwork_check_session` can return `logged_in: true` while `upwork_get_my_profile` and `upwork_get_connects_balance` simultaneously raise \"Not logged in to Upwork.\" Use `personal/career/upwork-profile-refresh-2026-05-01.md` (decisions now locked 2026-05-01T11:55+10:00, paste-ready copy includes Zendesk + ANZ named) as the trusted profile-update draft until selectors are repaired and the fix branch is installed.\n")
	b.WriteString("- `upwork_search_jobs` previously hit `/nx/find-work/best-matches`, which ignores `q=` and returns the same personalised feed for every query. The fork branch `nfsarch33/upwork-mcp:fix/search-url-skill-filter-profile-fallback` (commit `137231f`) switches it to `/nx/search/jobs`, filters skill UI noise, and reads `__NEXT_DATA__` for the profile. Push it before relying on Upwork search results.\n")
	b.WriteString("- Before first use after install or reboot, run the manual login/status flow outside Cursor, then restart Cursor so the MCP server descriptors load cleanly.\n\n")

	b.WriteString("## Browser-session MCP architecture variants\n")
	b.WriteString("- Prefer the LinkedIn-style production pattern for new platform MCPs: FastMCP lifespan, bootstrap readiness gate, persistent profile directory, sequential tool middleware, typed errors mapped to `ToolError`, masked diagnostics, POSIX file permissions, and broad unit tests.\n")
	b.WriteString("- Use the Upwork-style Chrome CDP pattern when managed browsers trigger anti-bot or Cloudflare flows: real Chrome, dedicated `--remote-debugging-port`, dedicated `--user-data-dir`, explicit `--login` / `--check` / `--logout`, and collision-resistant port env vars.\n")
	b.WriteString("- Any browser-session MCP backed by one profile must serialize tool execution or otherwise prevent concurrent page races.\n")
	b.WriteString("- Tool metadata should mark write/social actions with destructive hints where supported and repeat human-confirmation requirements in tool descriptions and governing skills.\n\n")

	b.WriteString("## Governing skill / rule paths\n")
	b.WriteString("- `context-mode` -> `context-mode` skill + `daily-startup-prompt.md` Phase 0\n")
	b.WriteString("- `mem0` -> `memory-system` or `memory-and-kb`\n")
	b.WriteString("- `git-mcp-server` -> repo rules + `github-identity` for identity-sensitive workflows\n")
	b.WriteString("- `github-official` -> `code-review-pro`, `gh-fix-ci`, or `github-release`\n")
	b.WriteString("- `context7` -> `context-hub`\n")
	b.WriteString("- `duckduckgo`, `perplexity`, `fetch`, `tavily`, `tavily-mcp`, `exa` -> `web-search-plus`\n")
	b.WriteString("- `wolfram-alpha` -> `skill-routing` math/calculation route\n")
	b.WriteString("- `google-scholar` -> `persona-researcher`, `research-pipeline`, and `academic-essay-writer` for citation-driven flows\n")
	b.WriteString("- `word-document-server` -> `pptx-mastery` / `academic-essay-writer` for long-form writing deliverables\n")
	b.WriteString("- `ironclaw` -> `ironclaw-mcp` bridge repo, `daily-startup-prompt.md`, `ironclaw/docs/LLM_PROVIDERS.md`, and the `llm-cluster-router` / `openclaw-vllm` skills\n")
	b.WriteString("- `linkedin-mcp` -> `linkedin-job-hunt`; browser profile login required; write actions need human confirmation\n")
	b.WriteString("- `upwork-mcp` -> `upwork-job-hunt`; Chrome CDP login required; proposal/message actions need human confirmation\n\n")

	b.WriteString("## Quality gates (non-breaking)\n")
	b.WriteString("- Default to non-breaking changes. Ask before breaking changes.\n")
	b.WriteString("- Before relying on a fresh install or resumed machine state: run `~/bin/cursor-tools doctor mcp`, `doctor platform`, and `selftest`.\n\n")

	b.WriteString("## Config hygiene\n")
	b.WriteString("- Local dev may keep static creds in `~/.cursor/mcp.json` if needed.\n")
	b.WriteString("- Never commit secrets into repos or rule files.\n")
	b.WriteString("- Keep MCP config entries redacted in docs. For browser-session MCPs, document only launch commands and session-directory risk, not cookies, profile contents, or account data.\n\n")

	b.WriteString("## Available MCP servers (redacted)\n\n")

	names := make([]string, 0, len(servers))
	for name := range servers {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		spec := servers[name]
		b.WriteString("### " + name + "\n")
		if spec.Disabled {
			b.WriteString("- status: `disabled`\n")
			if name == "allPepper-memory-bank" {
				b.WriteString("- note: legacy fallback only; do not use for active memory work in the Mem0-first model\n")
			}
		} else {
			b.WriteString("- status: `enabled`\n")
		}
		if spec.Type != "" {
			b.WriteString("- type: `" + spec.Type + "`\n")
		}
		if spec.URL != "" {
			b.WriteString("- url: `" + redactURLSecrets(spec.URL) + "`\n")
		}
		if spec.Command != "" {
			b.WriteString("- command: `" + spec.Command + "`\n")
			safeArgs := redactArgs(spec.Args)
			b.WriteString(fmt.Sprintf("- args: `%v`\n", safeArgs))
		} else if spec.URL != "" {
			b.WriteString("- command: `(HTTP / remote MCP — no local process)`\n")
		} else {
			b.WriteString("- command: `" + spec.Command + "`\n")
			safeArgs := redactArgs(spec.Args)
			b.WriteString(fmt.Sprintf("- args: `%v`\n", safeArgs))
		}
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

// stripTimestamp removes volatile timestamp lines for comparison purposes.
func stripTimestamp(s string) string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		if !strings.HasPrefix(line, "Last generated:") && !strings.HasPrefix(line, "Last reviewed:") {
			lines = append(lines, line)
		}
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

func runMCPIndex(_ *cobra.Command, _ []string) error {
	if mcpIndexFlags.check && mcpIndexFlags.write {
		return fmt.Errorf("--check and --write are mutually exclusive")
	}
	if mcpIndexFlags.check {
		current, err := os.ReadFile(mcpIndexFlags.out)
		if err != nil {
			return fmt.Errorf("reading existing MCP index: %w", err)
		}
		servers, err := loadMCPServers(mcpIndexFlags.mcpJSON)
		if err != nil {
			return err
		}
		rendered := renderMCPIndex(servers)
		if stripTimestamp(string(current)) != stripTimestamp(rendered) {
			return fmt.Errorf("MCP index is stale: run cursor-tools mcp-index --write")
		}
		clilog.Success("MCP index is current: %s", mcpIndexFlags.out)
		return nil
	}

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
