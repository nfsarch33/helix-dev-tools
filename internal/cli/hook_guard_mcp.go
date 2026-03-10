package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/cursor-tools/internal/config"
	"github.com/nfsarch33/cursor-tools/internal/hookio"
	"github.com/nfsarch33/cursor-tools/internal/logger"
	"github.com/nfsarch33/cursor-tools/internal/metrics"
	"github.com/nfsarch33/cursor-tools/internal/patterns"
)

// mcpToolServerMap maps known MCP tool names to their server as a fallback
// when Cursor does not send the server_name field.
var mcpToolServerMap = map[string]string{
	// context7
	"resolve-library-id": "context7",
	"query-docs":         "context7",
	"get-library-docs":   "context7",
	// duckduckgo
	"search":        "duckduckgo",
	"fetch_content": "duckduckgo",
	// perplexity-ask
	"perplexity_ask": "perplexity-ask",
	// fetch
	"fetch": "fetch",
	// playwright
	"browser_click":           "playwright",
	"browser_navigate":        "playwright",
	"browser_snapshot":        "playwright",
	"browser_type":            "playwright",
	"browser_fill_form":       "playwright",
	"browser_tabs":            "playwright",
	"browser_install":         "playwright",
	"browser_take_screenshot": "playwright",
	"browser_close":           "playwright",
	"browser_press_key":       "playwright",
	"browser_hover":           "playwright",
	"browser_select_option":   "playwright",
	"browser_navigate_back":   "playwright",
	"browser_evaluate":        "playwright",
	// chrome-devtools (legacy tool names from older sessions)
	"navigate_page":         "chrome-devtools",
	"take_screenshot":       "chrome-devtools",
	"click":                 "chrome-devtools",
	"fill":                  "chrome-devtools",
	"select_page":           "chrome-devtools",
	"type_text":             "chrome-devtools",
	"new_page":              "chrome-devtools",
	"list_pages":            "chrome-devtools",
	"evaluate_script":       "chrome-devtools",
	"take_snapshot":         "chrome-devtools",
	"list_network_requests": "chrome-devtools",
	"list_console_messages": "chrome-devtools",
	// github-official
	"get_toolset_tools":       "github-official",
	"enable_toolset":          "github-official",
	"list_tools":              "github-official",
	"list_available_toolsets": "github-official",
	// wolfram-alpha
	"query-wolfram-alpha": "wolfram-alpha",
	// time
	"get_current_time": "time",
	"convert_time":     "time",
	// sequential-thinking
	"sequential_thinking": "sequential-thinking",
	"analyze_problem":     "sequential-thinking",
	"problem_breakdown":   "sequential-thinking",
	"step_by_step_plan":   "sequential-thinking",
	// context-mode
	"ctx_execute":         "context-mode",
	"ctx_batch_execute":   "context-mode",
	"ctx_index":           "context-mode",
	"ctx_search":          "context-mode",
	"ctx_execute_file":    "context-mode",
	"ctx_fetch_and_index": "context-mode",
	"ctx_stats":           "context-mode",
	"ctx_doctor":          "context-mode",
	"ctx_upgrade":         "context-mode",
	// allPepper-memory-bank
	"memory_bank_read":   "allPepper-memory-bank",
	"memory_bank_write":  "allPepper-memory-bank",
	"memory_bank_update": "allPepper-memory-bank",
	"list_projects":      "allPepper-memory-bank",
	"list_project_files": "allPepper-memory-bank",
	// git-mcp-server (prefix-based tools, map the most common)
	"git_status":              "git-mcp-server",
	"git_diff":                "git-mcp-server",
	"git_log":                 "git-mcp-server",
	"git_commit":              "git-mcp-server",
	"git_push":                "git-mcp-server",
	"git_pull":                "git-mcp-server",
	"git_add":                 "git-mcp-server",
	"git_branch":              "git-mcp-server",
	"git_checkout":            "git-mcp-server",
	"git_blame":               "git-mcp-server",
	"git_stash":               "git-mcp-server",
	"git_fetch":               "git-mcp-server",
	"git_merge":               "git-mcp-server",
	"git_rebase":              "git-mcp-server",
	"git_tag":                 "git-mcp-server",
	"git_show":                "git-mcp-server",
	"git_remote":              "git-mcp-server",
	"git_reset":               "git-mcp-server",
	"git_clean":               "git-mcp-server",
	"git_clone":               "git-mcp-server",
	"git_init":                "git-mcp-server",
	"git_cherry_pick":         "git-mcp-server",
	"git_reflog":              "git-mcp-server",
	"git_worktree":            "git-mcp-server",
	"git_set_working_dir":     "git-mcp-server",
	"git_clear_working_dir":   "git-mcp-server",
	"git_changelog_analyze":   "git-mcp-server",
	"git_wrapup_instructions": "git-mcp-server",
}

var guardMcpCmd = &cobra.Command{
	Use:   "guard-mcp",
	Short: "beforeMCPExecution: gate destructive MCP tools",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGuardMcp(os.Stdin, os.Stdout)
	},
}

type guardMcpHandler struct {
	log         *logger.Logger
	metricsPath string
}

func (h *guardMcpHandler) Handle(_ context.Context, input *hookio.Input) (*hookio.Response, error) {
	start := time.Now()
	if input.ToolName == "" {
		return &hookio.Response{Permission: "allow"}, nil
	}

	toolInputShort := input.ToolInput
	if len(toolInputShort) > 100 {
		toolInputShort = toolInputShort[:100]
	}
	h.log.Log(fmt.Sprintf("MCP: %s input=%q", input.ToolName, toolInputShort))

	var actionStr string
	var resp *hookio.Response

	if patterns.MatchExact(input.ToolName, patterns.MCPDenyTools) {
		actionStr = "deny"
		resp = hookio.Deny(
			fmt.Sprintf("BLOCKED: MCP tool '%s' is destructive", input.ToolName),
			fmt.Sprintf("Tool '%s' is blocked by guard-mcp hook. Use a non-destructive alternative.", input.ToolName),
		)
	} else if patterns.MatchExact(input.ToolName, patterns.MCPWarnTools) {
		actionStr = "warn"
		resp = hookio.Ask(
			fmt.Sprintf("MCP '%s' modifies state. Confirm?", input.ToolName),
			fmt.Sprintf("Tool '%s' requires user confirmation as it modifies external state.", input.ToolName),
		)
	} else {
		actionStr = "allow"
		resp = &hookio.Response{Permission: "allow"}
	}

	// Resolve MCP server name: prefer Cursor-provided, fall back to static map
	serverName := input.ServerName
	if serverName == "" {
		serverName = mcpToolServerMap[input.ToolName]
	}
	detail := input.ToolName
	if serverName != "" {
		detail = serverName + ":" + input.ToolName
	}

	_ = metrics.Record(h.metricsPath, metrics.Event{
		Hook:      "guard-mcp",
		Action:    actionStr,
		Category:  "mcp",
		LatencyMs: time.Since(start).Milliseconds(),
		Detail:    detail,
		BytesIn:   int64(len(input.ToolName) + len(input.ToolInput)),
	})

	return resp, nil
}

func runGuardMcp(stdin *os.File, stdout *os.File) error {
	paths := config.DefaultPaths()
	handler := &guardMcpHandler{
		log:         logger.New(paths.LogFile("mcp-audit")),
		metricsPath: paths.MetricsFile(),
	}

	input, err := hookio.ReadInput(stdin)
	if err != nil {
		_ = hookio.WriteResponse(stdout, &hookio.Response{Permission: "allow"})
		return nil
	}

	resp, err := handler.Handle(context.Background(), input)
	if err != nil {
		_ = hookio.WriteResponse(stdout, &hookio.Response{Permission: "allow"})
		return nil
	}

	_ = hookio.WriteResponse(stdout, resp)
	if resp.Permission == "deny" {
		os.Exit(2)
	}
	return nil
}
