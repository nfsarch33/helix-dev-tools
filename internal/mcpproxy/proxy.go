// Package mcpproxy is a generic stdio JSON-RPC pass-through that sits between
// a JSON-RPC 2.0 MCP client (e.g. Cursor, Claude Code) and an upstream MCP
// server. Every line flowing in either direction is forwarded verbatim;
// structured lines matching the JSON-RPC tools/call request/response shape
// are also recorded as agentrace NDJSON events for downstream analytics.
//
// This package is the shared core for all MCP proxy binaries. The single
// cmd/mcp-proxy binary wraps any MCP server with agentrace logging via the
// --name flag, eliminating per-server proxy binaries.
package mcpproxy

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Config wires the four stdio streams plus log destination and labelling
// metadata. All Reader/Writer fields are mandatory; Now defaults to
// time.Now if nil.
type Config struct {
	ClientReader io.Reader
	ClientWriter io.Writer
	ServerReader io.Reader
	ServerWriter io.Writer

	LogPath string
	AgentID string
	Server  string

	Now func() time.Time
}

// Proxy is the running pass-through. Construct via New, drive via Run.
type Proxy struct {
	cfg Config
	log *logger
}

// New returns a Proxy bound to cfg. The agentrace log file is opened
// lazily inside Run so that filesystem errors surface alongside other
// IO errors and cannot corrupt construction.
func New(cfg Config) *Proxy {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Proxy{cfg: cfg}
}

// Run pumps client->server and server->client until either reader returns
// EOF, the context is cancelled, or an unrecoverable write error occurs.
// io.EOF and context.Canceled never propagate as errors.
func (p *Proxy) Run(ctx context.Context) error {
	lg, err := newLogger(p.cfg.LogPath)
	if err != nil {
		return fmt.Errorf("mcpproxy: open log: %w", err)
	}
	defer lg.Close()
	p.log = lg

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		<-runCtx.Done()
		if c, ok := p.cfg.ClientReader.(io.Closer); ok {
			_ = c.Close()
		}
		if c, ok := p.cfg.ServerReader.(io.Closer); ok {
			_ = c.Close()
		}
	}()

	pumpDone := make(chan error, 2)
	go func() {
		err := p.pumpClientToServer(runCtx)
		if c, ok := p.cfg.ServerWriter.(io.Closer); ok {
			_ = c.Close()
		}
		pumpDone <- err
	}()
	go func() { pumpDone <- p.pumpServerToClient(runCtx) }()

	var firstErr error
	for received := 0; received < 2; {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-pumpDone:
			received++
			if err != nil &&
				!errors.Is(err, io.EOF) &&
				!errors.Is(err, io.ErrClosedPipe) &&
				!errors.Is(err, context.Canceled) &&
				firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

const maxLineBytes = 4 * 1024 * 1024

func (p *Proxy) pumpClientToServer(ctx context.Context) error {
	return p.pump(ctx, p.cfg.ClientReader, p.cfg.ServerWriter, p.logRequest)
}

func (p *Proxy) pumpServerToClient(ctx context.Context) error {
	return p.pump(ctx, p.cfg.ServerReader, p.cfg.ClientWriter, p.logResponse)
}

func (p *Proxy) pump(ctx context.Context, src io.Reader, dst io.Writer, logFn func([]byte)) error {
	sc := bufio.NewScanner(src)
	sc.Buffer(make([]byte, 64*1024), maxLineBytes)
	for sc.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		line := sc.Bytes()
		out := make([]byte, 0, len(line)+1)
		out = append(out, line...)
		out = append(out, '\n')
		if _, err := dst.Write(out); err != nil {
			return err
		}
		logFn(line)
	}
	return sc.Err()
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  *struct {
		Name string `json:"name"`
	} `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (p *Proxy) logRequest(line []byte) {
	var req rpcRequest
	if err := json.Unmarshal(line, &req); err != nil {
		return
	}
	if req.Method == "" {
		return
	}
	ev := Event{
		Timestamp: p.cfg.Now().UTC().Format(time.RFC3339Nano),
		EventType: "mcp_request",
		Server:    p.cfg.Server,
		AgentID:   p.cfg.AgentID,
		RPCID:     idToString(req.ID),
		Method:    req.Method,
	}
	if req.Method == "tools/call" && req.Params != nil {
		ev.Tool = req.Params.Name
	}
	_ = p.log.write(ev)
}

func (p *Proxy) logResponse(line []byte) {
	var resp rpcResponse
	if err := json.Unmarshal(line, &resp); err != nil {
		return
	}
	if len(resp.ID) == 0 || string(resp.ID) == "null" {
		return
	}
	if resp.Result == nil && resp.Error == nil {
		return
	}
	ev := Event{
		Timestamp: p.cfg.Now().UTC().Format(time.RFC3339Nano),
		EventType: "mcp_response",
		Server:    p.cfg.Server,
		AgentID:   p.cfg.AgentID,
		RPCID:     idToString(resp.ID),
		Success:   resp.Error == nil,
	}
	if resp.Error != nil {
		ev.ErrorMessage = resp.Error.Message
	}
	_ = p.log.write(ev)
}

func idToString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return ""
	}
	var asStr string
	if err := json.Unmarshal(raw, &asStr); err == nil {
		return asStr
	}
	var asNum float64
	if err := json.Unmarshal(raw, &asNum); err == nil {
		if asNum == float64(int64(asNum)) {
			return strconv.FormatInt(int64(asNum), 10)
		}
		return strconv.FormatFloat(asNum, 'f', -1, 64)
	}
	return trimmed
}

// Event is one agentrace NDJSON record. Exported so CLI callers can
// reference the schema for documentation or validation.
type Event struct {
	Timestamp    string `json:"ts"`
	EventType    string `json:"event_type"`
	Tool         string `json:"tool,omitempty"`
	Server       string `json:"server,omitempty"`
	AgentID      string `json:"agent_id,omitempty"`
	RPCID        string `json:"rpc_id,omitempty"`
	Method       string `json:"method,omitempty"`
	Success      bool   `json:"success,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type logger struct {
	mu sync.Mutex
	f  *os.File
}

func newLogger(path string) (*logger, error) {
	if path == "" {
		return nil, errors.New("mcpproxy: log path required")
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return &logger{f: f}, nil
}

func (l *logger) write(ev Event) error {
	if l == nil {
		return nil
	}
	if ev.Timestamp == "" {
		ev.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	line, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	line = append(line, '\n')
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil {
		return nil
	}
	_, err = l.f.Write(line)
	return err
}

func (l *logger) Close() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil {
		return nil
	}
	err := l.f.Close()
	l.f = nil
	return err
}
