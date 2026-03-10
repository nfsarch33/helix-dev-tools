package metrics

// MCPToolServerMap maps known MCP tool names to their server as a fallback
// when the event's Detail field contains only the tool name (no "server:" prefix).
// Used for both live enrichment (guard-mcp hook) and retroactive enrichment
// (metrics summarisation of historical events).
var MCPToolServerMap = map[string]string{
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
	// chrome-devtools
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
	"update_pull_request":     "github-official",
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
	// git-mcp-server
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

// EnrichToolDetail takes an MCP event detail string and returns an enriched
// "server:tool" format. If already enriched (contains ":"), returns as-is.
func EnrichToolDetail(detail string) string {
	for i := 0; i < len(detail); i++ {
		if detail[i] == ':' {
			return detail
		}
	}
	if server, ok := MCPToolServerMap[detail]; ok {
		return server + ":" + detail
	}
	return detail
}
