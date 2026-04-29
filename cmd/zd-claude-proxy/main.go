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
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
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
		log.Printf("zd-claude-proxy probe: secret resolution OK")
		return
	}
	if opts.probeLive {
		switch opts.probeTarget {
		case "bedrock", "":
			if err := probeLiveBedrock(ctx, cfg, secrets, opts.probeModel); err != nil {
				fatal("probe-live (bedrock): %v", err)
			}
		case "openai":
			if err := probeLiveOpenAIChat(ctx, cfg, secrets, opts.probeModel); err != nil {
				fatal("probe-live (openai): %v", err)
			}
		default:
			fatal("probe-live: unknown target %q (expected bedrock or openai)", opts.probeTarget)
		}
		return
	}

	tokenPath := cfg.LocalTokenPath
	if tokenPath == "" {
		tokenPath, err = zdproxy.DefaultLocalTokenPath()
		if err != nil {
			fatal("resolve default token path: %v", err)
		}
	}
	localToken, written, err := zdproxy.ResolveLocalToken(tokenPath, opts.reuseLocalToken)
	if err != nil {
		fatal("resolve local token: %v", err)
	}
	if written {
		log.Printf("zd-claude-proxy: local-token file written to %s (mode 0600)", tokenPath)
	} else {
		log.Printf("zd-claude-proxy: reusing existing local-token at %s (rotate with --reuse-local-token=false)", tokenPath)
	}

	srv, err := zdproxy.NewServer(cfg, secrets, localToken)
	if err != nil {
		fatal("new server: %v", err)
	}
	if err := srv.Start(ctx); err != nil {
		fatal("start server: %v", err)
	}
	log.Printf("zd-claude-proxy: listening on http://%s (loopback only)", srv.Addr())
	log.Printf("zd-claude-proxy: routes:")
	log.Printf("  POST /v1/messages                                          (auth) Anthropic Messages, model-aware dispatch (Claude->Bedrock; GPT/o3/o4/codex->OpenAI)")
	log.Printf("  POST /v1/chat/completions                                  (auth) OpenAI Chat Completions passthrough (Cursor)")
	log.Printf("  POST /v1/responses                                         (auth) OpenAI Responses passthrough (codex/o3/o4/pro)")
	log.Printf("  POST /bedrock/model/{id}/invoke                            (auth) Bedrock-shape passthrough")
	log.Printf("  POST /bedrock/model/{id}/invoke-with-response-stream       (auth) Bedrock streaming passthrough")
	log.Printf("  GET  /healthz                                              (no auth) readiness check")
	log.Printf("  GET  /version                                              (no auth) build metadata")
	log.Printf("zd-claude-proxy: auth (canonical): `X-Local-Auth: Bearer $(cat %s)`", tokenPath)
	log.Printf("zd-claude-proxy: auth (fallback for OpenAI clients): `Authorization: Bearer $(cat %s)`", tokenPath)
	log.Printf("zd-claude-proxy: bedrock=%s openai=%s (tokens redacted)", cfg.BedrockBaseURL, cfg.OpenAIBaseURL)

	<-ctx.Done()
	log.Printf("zd-claude-proxy: shutdown signal received, stopping...")
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	_ = srv.Stop(shutdownCtx)
	if !opts.reuseLocalToken {
		// Strict rotation mode: clean up the token file so the next
		// boot is guaranteed to mint a fresh one. In reuse mode the
		// file is intentionally left in place so long-running clients
		// (Claude Desktop, Claude CLI, Codex CLI) survive the restart.
		_ = os.Remove(tokenPath)
		log.Printf("zd-claude-proxy: removed local-token file (rotate mode)")
	} else {
		log.Printf("zd-claude-proxy: kept local-token file at %s for next boot (reuse mode)", tokenPath)
	}
	log.Printf("zd-claude-proxy: stopped")
}

type runtimeOptions struct {
	probe           bool
	probeLive       bool
	probeModel      string
	probeTarget     string
	showVersion     bool
	opVault         string
	reuseLocalToken bool
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
	fs.StringVar(&cfg.OpBedrockField, "op-bedrock-field", "AWS_BEARER_TOKEN_BEDROCK", "shell env-var name in the bedrock notesPlain snippet (empty = treat whole notesPlain as bearer)")
	fs.StringVar(&cfg.OpOpenAIItem, "op-openai-item", "zd api gateway openai models", "1Password item title carrying the OpenAI bearer in notesPlain")
	fs.StringVar(&cfg.OpOpenAIField, "op-openai-field", "OPENAI_API_KEY", "shell env-var name in the openai notesPlain snippet (empty = treat whole notesPlain as bearer)")
	fs.StringVar(&cfg.LocalTokenPath, "local-token-path", "", "override path for the per-process local auth token (default: $XDG_CONFIG_HOME/zd-claude-proxy/local-token)")
	fs.StringVar(&opts.opVault, "op-vault", "Cursor_IronClaw", "1Password vault name to scope item lookups to")
	fs.BoolVar(&opts.probe, "probe", false, "resolve secrets via op, log redacted lengths, and exit (cheap smoke gate; no gateway request)")
	fs.BoolVar(&opts.probeLive, "probe-live", false, "live end-to-end probe: send a tiny request through the configured gateway and exit (no listener bound)")
	fs.StringVar(&opts.probeModel, "probe-model", "us.anthropic.claude-3-5-haiku-20241022-v1:0", "model id to use for --probe-live (default: cheapest Haiku)")
	fs.StringVar(&opts.probeTarget, "probe-target", "bedrock", "--probe-live target: `bedrock` (default) or `openai`")
	fs.BoolVar(&opts.showVersion, "version", false, "print version and exit")
	fs.BoolVar(&opts.reuseLocalToken, "reuse-local-token", true, "if true, reuse the existing local-token file when its content is a valid 32-byte base64url token; otherwise mint+rotate on every boot")

	_ = fs.Parse(os.Args[1:])
	return cfg, opts
}

func fatal(format string, args ...any) {
	log.Printf("zd-claude-proxy: fatal: "+format, args...)
	os.Exit(1)
}

// probeLiveBedrock sends a single tiny Anthropic Messages request directly to
// the configured ZD Bedrock gateway (bypassing the proxy listener) to prove
// that VPN + 1Password-resolved bearer + gateway routing all work
// end-to-end. Output is redacted (no bearer ever logged); only HTTP status +
// a one-line usage summary is printed.
func probeLiveBedrock(ctx context.Context, cfg zdproxy.Config, secrets zdproxy.Secrets, model string) error {
	// Bedrock invoke takes the model in the URL path, NOT the body.
	body := map[string]any{
		"anthropic_version": "bedrock-2023-05-31",
		"max_tokens":        8,
		"messages": []map[string]any{
			{"role": "user", "content": "ping"},
		},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal body: %w", err)
	}
	url := fmt.Sprintf("%s/model/%s/invoke", cfg.BedrockBaseURL, model)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+secrets.BedrockBearer)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("upstream POST: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("upstream HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	var parsed struct {
		ID    string `json:"id"`
		Type  string `json:"type"`
		Model string `json:"model"`
		Usage struct {
			Input  int `json:"input_tokens"`
			Output int `json:"output_tokens"`
		} `json:"usage"`
	}
	_ = json.Unmarshal(respBody, &parsed)
	log.Printf("zd-claude-proxy probe-live (bedrock): HTTP %d model=%s id=%s usage.input=%d usage.output=%d",
		resp.StatusCode, parsed.Model, parsed.ID, parsed.Usage.Input, parsed.Usage.Output)
	return nil
}

// probeLiveOpenAIChat sends a single tiny OpenAI Chat Completions request
// directly to the configured ZD OpenAI gateway. Defaults the model to
// gpt-5.5 when the caller did not pass --probe-model (or passed the Bedrock
// default). Output is redacted; only HTTP status + usage summary is logged.
func probeLiveOpenAIChat(ctx context.Context, cfg zdproxy.Config, secrets zdproxy.Secrets, model string) error {
	if model == "" || model == "us.anthropic.claude-3-5-haiku-20241022-v1:0" {
		model = "gpt-5.5"
	}
	body := map[string]any{
		"model":                 model,
		"max_completion_tokens": 8,
		"messages": []map[string]any{
			{"role": "user", "content": "ping"},
		},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal body: %w", err)
	}
	url := cfg.OpenAIBaseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+secrets.OpenAIBearer)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("upstream POST: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("upstream HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	var parsed struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		Usage struct {
			Prompt     int `json:"prompt_tokens"`
			Completion int `json:"completion_tokens"`
		} `json:"usage"`
		Choices []struct {
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	_ = json.Unmarshal(respBody, &parsed)
	finish := ""
	if len(parsed.Choices) > 0 {
		finish = parsed.Choices[0].FinishReason
	}
	log.Printf("zd-claude-proxy probe-live (openai): HTTP %d model=%s id=%s usage.prompt=%d usage.completion=%d finish=%s",
		resp.StatusCode, parsed.Model, parsed.ID, parsed.Usage.Prompt, parsed.Usage.Completion, finish)
	return nil
}
