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
	// perplexity
	"perplexity_ask":      "perplexity",
	"perplexity_search":   "perplexity",
	"perplexity_research": "perplexity",
	"perplexity_reason":   "perplexity",
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
	// mem0
	"add_memory":          "mem0",
	"search_memories":     "mem0",
	"get_memories":        "mem0",
	"get_memory":          "mem0",
	"update_memory":       "mem0",
	"delete_memory":       "mem0",
	"delete_all_memories": "mem0",
	"delete_entities":     "mem0",
	"list_entities":       "mem0",
	// allPepper-memory-bank
	"memory_bank_read":   "allPepper-memory-bank",
	"memory_bank_write":  "allPepper-memory-bank",
	"memory_bank_update": "allPepper-memory-bank",
	"list_projects":      "allPepper-memory-bank",
	"list_project_files": "allPepper-memory-bank",
	// helixon (helixon-mcp bridge)
	"helixon_health":         "helixon",
	"helixon_chat":           "helixon",
	"helixon_list_jobs":      "helixon",
	"helixon_get_job":        "helixon",
	"helixon_cancel_job":     "helixon",
	"helixon_search_memory":  "helixon",
	"helixon_list_routines":  "helixon",
	"helixon_create_routine": "helixon",
	"helixon_delete_routine": "helixon",
	"helixon_list_tools":     "helixon",
	// atlassian-jira
	"jira_get_issue":                "atlassian-jira",
	"jira_create_issue":             "atlassian-jira",
	"jira_update_issue":             "atlassian-jira",
	"jira_search":                   "atlassian-jira",
	"jira_add_comment":              "atlassian-jira",
	"jira_transition_issue":         "atlassian-jira",
	"jira_get_all_projects":         "atlassian-jira",
	"jira_get_project_issues":       "atlassian-jira",
	"jira_get_transitions":          "atlassian-jira",
	"jira_add_worklog":              "atlassian-jira",
	"jira_get_worklog":              "atlassian-jira",
	"jira_create_sprint":            "atlassian-jira",
	"jira_get_agile_boards":         "atlassian-jira",
	"jira_get_board_issues":         "atlassian-jira",
	"jira_get_sprint_issues":        "atlassian-jira",
	"jira_get_sprints_from_board":   "atlassian-jira",
	"jira_get_project_versions":     "atlassian-jira",
	"jira_create_version":           "atlassian-jira",
	"jira_batch_create_issues":      "atlassian-jira",
	"jira_batch_create_versions":    "atlassian-jira",
	"jira_batch_get_changelogs":     "atlassian-jira",
	"jira_create_issue_link":        "atlassian-jira",
	"jira_create_remote_issue_link": "atlassian-jira",
	"jira_delete_issue":             "atlassian-jira",
	"jira_download_attachments":     "atlassian-jira",
	"jira_get_link_types":           "atlassian-jira",
	"jira_get_user_profile":         "atlassian-jira",
	"jira_link_to_epic":             "atlassian-jira",
	"jira_remove_issue_link":        "atlassian-jira",
	"jira_search_fields":            "atlassian-jira",
	// tavily-mcp
	"tavily_search":   "tavily-mcp",
	"tavily_extract":  "tavily-mcp",
	"tavily_crawl":    "tavily-mcp",
	"tavily_map":      "tavily-mcp",
	"tavily_research": "tavily-mcp",
	// exa
	"web_search_exa": "exa",
	"web_fetch_exa":  "exa",
	// pdf-handler
	"read_pdf":                "pdf-handler",
	"merge_pdfs":              "pdf-handler",
	"split_pdf":               "pdf-handler",
	"rotate_pages":            "pdf-handler",
	"extract_text":            "pdf-handler",
	"extract_tables":          "pdf-handler",
	"extract_images":          "pdf-handler",
	"extract_links":           "pdf-handler",
	"extract_pages":           "pdf-handler",
	"extract_structured_data": "pdf-handler",
	"get_pdf_metadata":        "pdf-handler",
	"fill_pdf_form":           "pdf-handler",
	"fill_pdf_form_any":       "pdf-handler",
	"get_pdf_form_fields":     "pdf-handler",
	"encrypt_pdf":             "pdf-handler",
	"optimize_pdf":            "pdf-handler",
	"compare_pdfs":            "pdf-handler",
	"export_pdf":              "pdf-handler",
	"analyze_pdf_content":     "pdf-handler",
	"detect_pdf_type":         "pdf-handler",
	// word-document-server
	"create_document":  "word-document-server",
	"edit_document":    "word-document-server",
	"convert_document": "word-document-server",
	// plantuml
	"generate_plantuml": "plantuml",
	// mermaid
	"generate_mermaid_diagram": "mermaid",
	// huggingface
	"model_search":   "huggingface",
	"space_search":   "huggingface",
	"dataset_search": "huggingface",
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
			server := CanonicalMCPServerName(detail[:i])
			return server + detail[i:]
		}
	}
	if server, ok := MCPToolServerMap[detail]; ok {
		return CanonicalMCPServerName(server) + ":" + detail
	}
	return detail
}

// CanonicalMCPServerName collapses legacy aliases into a single reporting name.
func CanonicalMCPServerName(name string) string {
	switch name {
	case "perplexity-ask":
		return "perplexity"
	default:
		return name
	}
}
