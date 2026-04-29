package zdproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// Server is the loopback HTTP server. It exposes:
//
//   - GET  /healthz                                          (no auth) — readiness check.
//   - GET  /version                                          (no auth) — build metadata.
//   - POST /v1/messages                                      (auth)   — Anthropic Messages, model-aware dispatch
//     to Bedrock /invoke (Claude family) or OpenAI /v1/chat/completions /
//     /v1/responses (GPT / o3 / o4 / codex / pro families).
//   - POST /v1/chat/completions                              (auth)   — OpenAI Chat Completions passthrough.
//   - POST /v1/responses                                     (auth)   — OpenAI Responses passthrough.
//   - POST /bedrock/model/{id}/invoke                        (auth)   — Bedrock-shape passthrough.
//   - POST /bedrock/model/{id}/invoke-with-response-stream   (auth)   — Bedrock streaming passthrough.
//
// All authenticated handlers are wrapped with AuthMiddleware. The listener
// binds only to a loopback address; NewServer refuses any other.
//
// Metrics are emitted on a second loopback listener (cfg.MetricsBind) at
// /metrics in Prometheus text exposition format.
type Server struct {
	cfg        Config
	secrets    Secrets
	localToken string
	bedrock    *BedrockTransport
	openai     *OpenAITransport
	dispatcher *MessagesDispatcher
	metrics    *Metrics

	httpServer    *http.Server
	listener      net.Listener
	addr          string
	metricsServer *http.Server
	metricsLn     net.Listener
	metricsAddr   string
	mu            sync.Mutex
}

// NewServer constructs a Server, validating the loopback invariants. The
// returned Server is not yet listening — call Start to bind the listener.
func NewServer(cfg Config, secrets Secrets, localToken string) (*Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	if localToken == "" {
		return nil, fmt.Errorf("local token must be non-empty")
	}
	metrics := NewMetrics()
	bedrock := NewBedrockTransport(cfg.BedrockBaseURL, secrets.BedrockBearer, nil)
	bedrock.Metrics = metrics
	openai := NewOpenAITransport(cfg.OpenAIBaseURL, secrets.OpenAIBearer, nil)
	openai.Metrics = metrics
	return &Server{
		cfg:        cfg,
		secrets:    secrets,
		localToken: localToken,
		bedrock:    bedrock,
		openai:     openai,
		dispatcher: &MessagesDispatcher{Bedrock: bedrock, OpenAI: openai},
		metrics:    metrics,
	}, nil
}

// Start binds the listener on the configured loopback address and begins
// serving. The method is non-blocking; the caller must invoke Stop on
// shutdown.
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.httpServer != nil {
		return fmt.Errorf("server already started")
	}
	ln, err := net.Listen("tcp", s.cfg.Bind)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.cfg.Bind, err)
	}
	s.listener = ln
	s.addr = ln.Addr().String()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/version", s.handleVersion)
	auth := AuthMiddleware(s.localToken, s.secrets.BedrockBearer, s.secrets.OpenAIBearer)
	mux.Handle("/v1/messages", auth(http.HandlerFunc(s.dispatcher.HandleAnthropicMessages)))
	mux.Handle("/v1/chat/completions", auth(http.HandlerFunc(s.openai.HandleChatCompletions)))
	mux.Handle("/v1/responses", auth(http.HandlerFunc(s.openai.HandleResponses)))
	mux.Handle("/bedrock/", auth(http.HandlerFunc(s.bedrock.HandleBedrockPassthrough)))

	s.httpServer = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		_ = s.httpServer.Serve(ln)
	}()

	if s.cfg.MetricsBind != "" {
		mln, err := net.Listen("tcp", s.cfg.MetricsBind)
		if err != nil {
			_ = s.httpServer.Shutdown(context.Background())
			s.httpServer = nil
			return fmt.Errorf("listen metrics %s: %w", s.cfg.MetricsBind, err)
		}
		s.metricsLn = mln
		s.metricsAddr = mln.Addr().String()
		mmux := http.NewServeMux()
		mmux.Handle("/metrics", s.metrics)
		mmux.HandleFunc("/healthz", s.handleHealthz)
		s.metricsServer = &http.Server{
			Handler:           mmux,
			ReadHeaderTimeout: 10 * time.Second,
		}
		go func() {
			_ = s.metricsServer.Serve(mln)
		}()
	}
	return nil
}

// Addr returns the bound address (e.g. "127.0.0.1:8767"). Empty before Start.
func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addr
}

// MetricsAddr returns the bound metrics-listener address (e.g.
// "127.0.0.1:9787"). Empty before Start or when MetricsBind is unset.
func (s *Server) MetricsAddr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.metricsAddr
}

// Stop performs a graceful shutdown of both the main and metrics listeners.
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.httpServer == nil {
		return nil
	}
	srv := s.httpServer
	s.httpServer = nil
	mainErr := srv.Shutdown(ctx)
	if s.metricsServer != nil {
		_ = s.metricsServer.Shutdown(ctx)
		s.metricsServer = nil
	}
	return mainErr
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleVersion(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"name":             "zd-claude-proxy",
		"surface":          "macbook-only",
		"bedrock_base_url": s.cfg.BedrockBaseURL,
		"openai_base_url":  s.cfg.OpenAIBaseURL,
		"local_listener":   s.addr,
	})
}

// SetBedrockTransport overrides the default BedrockTransport. Tests use it to
// point the proxy at an httptest.Server upstream without leaving the loopback
// invariants enforced on cfg.BedrockBaseURL.
func (s *Server) SetBedrockTransport(bt *BedrockTransport) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if bt != nil && bt.Metrics == nil {
		bt.Metrics = s.metrics
	}
	s.bedrock = bt
	if s.dispatcher != nil {
		s.dispatcher.Bedrock = bt
	}
}

// SetOpenAITransport overrides the default OpenAITransport. Tests use it to
// point the proxy at an httptest.Server upstream without leaving the loopback
// invariants enforced on cfg.OpenAIBaseURL.
func (s *Server) SetOpenAITransport(ot *OpenAITransport) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ot != nil && ot.Metrics == nil {
		ot.Metrics = s.metrics
	}
	s.openai = ot
	if s.dispatcher != nil {
		s.dispatcher.OpenAI = ot
	}
}
