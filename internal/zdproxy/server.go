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
//   - GET  /healthz  (no auth) — readiness check, returns {status: ok}.
//   - POST /messages (auth)    — Anthropic Messages translator. MVP returns
//     501 Not Implemented; v257 R&D spike fills in Bedrock + OpenAI shapes.
//   - GET  /version  (no auth) — build/version metadata.
//
// All handlers are wrapped with the AuthMiddleware where appropriate. The
// listener binds only to a loopback address; NewServer refuses any other.
type Server struct {
	cfg        Config
	secrets    Secrets
	localToken string

	httpServer *http.Server
	listener   net.Listener
	addr       string
	mu         sync.Mutex
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
	return &Server{cfg: cfg, secrets: secrets, localToken: localToken}, nil
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
	mux.Handle("/messages", AuthMiddleware(s.localToken)(http.HandlerFunc(s.handleMessages)))

	s.httpServer = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		_ = s.httpServer.Serve(ln)
	}()
	return nil
}

// Addr returns the bound address (e.g. "127.0.0.1:8767"). Empty before Start.
func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addr
}

// Stop performs a graceful shutdown.
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.httpServer == nil {
		return nil
	}
	srv := s.httpServer
	s.httpServer = nil
	return srv.Shutdown(ctx)
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

// handleMessages is the inbound Anthropic Messages handler. The MVP returns
// 501 Not Implemented to ship the security envelope (loopback bind, local
// auth token, no upstream credential leakage) before v257 plumbs in the
// Bedrock and OpenAI translators.
func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	_, _ = w.Write([]byte(`{"error":{"type":"not_implemented","message":"zd-claude-proxy MVP: translator scheduled for v257 R&D spike. See reports/research/claude-desktop-custom-base-url-2026-04-28.md."}}`))
}
