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
	"resolve-library-id":    "context7",
	"query-docs":            "context7",
	"get-library-docs":      "context7",
	"search":                "duckduckgo",
	"perplexity_ask":        "perplexity-ask",
	"fetch":                 "fetch",
	"navigate_page":         "playwright",
	"take_screenshot":       "playwright",
	"click":                 "playwright",
	"fill":                  "playwright",
	"select_page":           "playwright",
	"type_text":             "playwright",
	"new_page":              "playwright",
	"list_pages":            "chrome-devtools",
	"evaluate_script":       "chrome-devtools",
	"take_snapshot":         "chrome-devtools",
	"list_network_requests": "chrome-devtools",
	"list_console_messages": "chrome-devtools",
	"get_toolset_tools":     "github-official",
	"enable_toolset":        "github-official",
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
