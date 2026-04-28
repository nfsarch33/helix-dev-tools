// Command zd-claude-proxy runs a MacBook-local Anthropic-Messages translator
// that routes Claude Code CLI / Desktop Code-runtime children to the Zendesk
// AI gateway (Bedrock + OpenAI surfaces).
//
// Hard constraints (enforced at boot):
//   - listener binds only to a loopback address
//   - metrics listener binds only to a loopback address
//   - gateway base URLs must use https://
//   - upstream tokens are resolved from 1Password at startup; nothing on disk
//
// This binary is MacBook-only by repo-owner directive (v256 D28). It must
// never be deployed to the home fleet (llm-cluster-router, gstack
// research-agent, pdf-mcp-server, IronClaw runtime, mission-control, member
// DevOps/SysAdmin agents, Hermes, OpenClaw, or any Tailscale/OCI-attached
// node).
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nfsarch33/cursor-tools/internal/zdproxy"
)

var version = "dev"

func main() {
	cfg, opts := parseFlags()
	if opts.showVersion {
		fmt.Printf("zd-claude-proxy %s\n", version)
		return
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := cfg.Validate(); err != nil {
		fatal("config: %v", err)
	}

	resolver := &zdproxy.CLIOpResolver{Vault: opts.opVault}
	secrets, err := zdproxy.LoadSecrets(ctx, cfg, resolver)
	if err != nil {
		fatal("load secrets: %v", err)
	}

	if opts.probe {
		log.Printf("zd-claude-proxy probe: secrets resolved (bedrock len=%d openai len=%d)", len(secrets.BedrockBearer), len(secrets.OpenAIBearer))
		log.Printf("zd-claude-proxy probe: secret resolution OK; gateway HTTP probe is the v257 spike scope")
		return
	}

	tokenPath := cfg.LocalTokenPath
	if tokenPath == "" {
		tokenPath, err = zdproxy.DefaultLocalTokenPath()
		if err != nil {
			fatal("resolve default token path: %v", err)
		}
	}
	localToken, err := zdproxy.NewLocalToken()
	if err != nil {
		fatal("mint local token: %v", err)
	}
	if err := zdproxy.WriteLocalTokenFile(tokenPath, localToken); err != nil {
		fatal("write local token file: %v", err)
	}
	log.Printf("zd-claude-proxy: local-token file written to %s (mode 0600)", tokenPath)

	srv, err := zdproxy.NewServer(cfg, secrets, localToken)
	if err != nil {
		fatal("new server: %v", err)
	}
	if err := srv.Start(ctx); err != nil {
		fatal("start server: %v", err)
	}
	log.Printf("zd-claude-proxy: listening on http://%s/messages (loopback only); use header `X-Local-Auth: Bearer $(cat %s)`", srv.Addr(), tokenPath)
	log.Printf("zd-claude-proxy: bedrock=%s openai=%s (tokens redacted)", cfg.BedrockBaseURL, cfg.OpenAIBaseURL)

	<-ctx.Done()
	log.Printf("zd-claude-proxy: shutdown signal received, stopping...")
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	_ = srv.Stop(shutdownCtx)
	_ = os.Remove(tokenPath)
	log.Printf("zd-claude-proxy: stopped")
}

type runtimeOptions struct {
	probe       bool
	showVersion bool
	opVault     string
}

func parseFlags() (zdproxy.Config, runtimeOptions) {
	var cfg zdproxy.Config
	var opts runtimeOptions

	fs := flag.NewFlagSet("zd-claude-proxy", flag.ExitOnError)
	fs.StringVar(&cfg.Bind, "bind", "127.0.0.1:8767", "loopback host:port for the inbound Anthropic-Messages listener")
	fs.StringVar(&cfg.MetricsBind, "metrics", "127.0.0.1:9787", "loopback host:port for the Prometheus metrics listener")
	fs.StringVar(&cfg.BedrockBaseURL, "bedrock-base-url", "https://ai-gateway.zende.sk/bedrock", "ZD Bedrock gateway base URL (https://)")
	fs.StringVar(&cfg.OpenAIBaseURL, "openai-base-url", "https://ai-gateway.zende.sk/v1", "ZD OpenAI gateway base URL (https://)")
	fs.StringVar(&cfg.OpBedrockItem, "op-bedrock-item", "zd api gateway bedrock claude models", "1Password item title carrying the Bedrock bearer in notesPlain")
	fs.StringVar(&cfg.OpOpenAIItem, "op-openai-item", "zd api gateway openai models", "1Password item title carrying the OpenAI bearer in notesPlain")
	fs.StringVar(&cfg.LocalTokenPath, "local-token-path", "", "override path for the per-process local auth token (default: $XDG_CONFIG_HOME/zd-claude-proxy/local-token)")
	fs.StringVar(&opts.opVault, "op-vault", "Cursor_IronClaw", "1Password vault name to scope item lookups to")
	fs.BoolVar(&opts.probe, "probe", false, "resolve secrets via op, log redacted lengths, and exit (smoke gate; no listener bound)")
	fs.BoolVar(&opts.showVersion, "version", false, "print version and exit")

	_ = fs.Parse(os.Args[1:])
	return cfg, opts
}

func fatal(format string, args ...any) {
	log.Printf("zd-claude-proxy: fatal: "+format, args...)
	os.Exit(1)
}
