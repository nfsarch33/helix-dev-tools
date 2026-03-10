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

var guardMcpExit = os.Exit

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

	// Resolve MCP server name: prefer Cursor-provided, fall back to shared map
	serverName := input.ServerName
	if serverName == "" {
		serverName = metrics.MCPToolServerMap[input.ToolName]
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
		guardMcpExit(2)
	}
	return nil
}
