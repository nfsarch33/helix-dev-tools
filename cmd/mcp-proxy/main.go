// mcp-proxy is a generic MCP stdio proxy that wraps any MCP server binary
// with agentrace NDJSON logging. It forwards JSON-RPC lines bidirectionally
// and records tools/call request/response events.
//
// Usage:
//
//	mcp-proxy --log ~/logs/runx/agentrace-mcp.ndjson --name sentrux -- sentrux-binary args...
//	mcp-proxy --name mem0 -- uvx mem0-mcp
//
// Wire into ~/.cursor/mcp.json by replacing the upstream command with mcp-proxy:
//
//	{
//	  "mcpServers": {
//	    "sentrux": {
//	      "command": "mcp-proxy",
//	      "args": ["--name", "sentrux", "--", "sentrux-binary", "--repo", "."]
//	    }
//	  }
//	}
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/nfsarch33/helix-dev-tools/internal/mcpproxy"
)

const defaultLogPath = "${HOME}/logs/runx/agentrace-mcp.ndjson"

func main() {
	args := os.Args[1:]

	var logPath, name string
	var upstreamArgs []string
	logPath = os.ExpandEnv(defaultLogPath)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--log":
			if i+1 < len(args) {
				i++
				logPath = os.ExpandEnv(args[i])
			}
		case "--name":
			if i+1 < len(args) {
				i++
				name = args[i]
			}
		case "--":
			upstreamArgs = args[i+1:]
			i = len(args)
		default:
			upstreamArgs = args[i:]
			i = len(args)
		}
	}

	if len(upstreamArgs) == 0 {
		fmt.Fprintln(os.Stderr, "usage: mcp-proxy [--log PATH] [--name NAME] -- COMMAND [ARGS...]")
		os.Exit(2)
	}
	if name == "" {
		name = upstreamArgs[0]
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cmd := exec.CommandContext(ctx, upstreamArgs[0], upstreamArgs[1:]...)
	cmd.Stderr = os.Stderr

	upstreamStdin, err := cmd.StdinPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "mcp-proxy: stdin pipe: %v\n", err)
		os.Exit(1)
	}
	upstreamStdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "mcp-proxy: stdout pipe: %v\n", err)
		os.Exit(1)
	}

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "mcp-proxy: start %q: %v\n", upstreamArgs[0], err)
		os.Exit(1)
	}

	proxy := mcpproxy.New(mcpproxy.Config{
		ClientReader: os.Stdin,
		ClientWriter: os.Stdout,
		ServerReader: upstreamStdout,
		ServerWriter: upstreamStdin,
		LogPath:      logPath,
		AgentID:      "cursor-parent",
		Server:       name,
	})

	proxyErr := proxy.Run(ctx)

	_ = cmd.Wait()

	if proxyErr != nil {
		fmt.Fprintf(os.Stderr, "mcp-proxy: %v\n", proxyErr)
		os.Exit(1)
	}
}
