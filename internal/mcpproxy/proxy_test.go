package mcpproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func fixedTime() time.Time { return time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC) }

func TestProxy_PassesClientToServer(t *testing.T) {
	t.Parallel()
	clientIn := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"gate"}}` + "\n")
	var serverIn bytes.Buffer
	serverOut := strings.NewReader("")
	var clientOut bytes.Buffer

	p := New(Config{
		ClientReader: clientIn,
		ClientWriter: &clientOut,
		ServerReader: serverOut,
		ServerWriter: &serverIn,
		LogPath:      filepath.Join(t.TempDir(), "agentrace.ndjson"),
		AgentID:      "cursor-parent",
		Server:       "sentrux",
		Now:          fixedTime,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := p.Run(ctx); err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, context.Canceled) {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(serverIn.String(), "gate") {
		t.Fatalf("server did not receive client payload, got %q", serverIn.String())
	}
}

func TestProxy_PassesServerToClient(t *testing.T) {
	t.Parallel()
	clientIn := strings.NewReader("")
	var serverIn bytes.Buffer
	serverOut := strings.NewReader(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"ok"}]}}` + "\n")
	var clientOut bytes.Buffer

	p := New(Config{
		ClientReader: clientIn,
		ClientWriter: &clientOut,
		ServerReader: serverOut,
		ServerWriter: &serverIn,
		LogPath:      filepath.Join(t.TempDir(), "agentrace.ndjson"),
		AgentID:      "cursor-parent",
		Server:       "sentrux",
		Now:          fixedTime,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := p.Run(ctx); err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, context.Canceled) {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(clientOut.String(), `"result"`) {
		t.Fatalf("client did not receive server response, got %q", clientOut.String())
	}
}

func TestProxy_LogsToolCallRequest(t *testing.T) {
	t.Parallel()
	clientIn := strings.NewReader(
		`{"jsonrpc":"2.0","id":42,"method":"tools/call","params":{"name":"sentrux_scan","arguments":{"path":"."}}}` + "\n")
	var serverIn bytes.Buffer
	serverOut := strings.NewReader("")
	var clientOut bytes.Buffer

	logFile := filepath.Join(t.TempDir(), "agentrace.ndjson")
	p := New(Config{
		ClientReader: clientIn,
		ClientWriter: &clientOut,
		ServerReader: serverOut,
		ServerWriter: &serverIn,
		LogPath:      logFile,
		AgentID:      "cursor-parent",
		Server:       "sentrux",
		Now:          fixedTime,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = p.Run(ctx)

	raw, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatalf("expected at least one log line, got %d", len(lines))
	}

	var ev Event
	if err := json.Unmarshal([]byte(lines[0]), &ev); err != nil {
		t.Fatalf("unmarshal: %v (line=%q)", err, lines[0])
	}
	if ev.EventType != "mcp_request" {
		t.Errorf("event_type = %q, want mcp_request", ev.EventType)
	}
	if ev.Tool != "sentrux_scan" {
		t.Errorf("tool = %q, want sentrux_scan", ev.Tool)
	}
	if ev.Server != "sentrux" {
		t.Errorf("server = %q, want sentrux", ev.Server)
	}
	if ev.RPCID != "42" {
		t.Errorf("rpc_id = %q, want 42", ev.RPCID)
	}
	if ev.Method != "tools/call" {
		t.Errorf("method = %q, want tools/call", ev.Method)
	}
}

func TestProxy_LogsToolCallResponse(t *testing.T) {
	t.Parallel()
	clientIn := strings.NewReader("")
	var serverIn bytes.Buffer
	serverOut := strings.NewReader(
		`{"jsonrpc":"2.0","id":42,"result":{"content":[{"type":"text","text":"pass"}]}}` + "\n")
	var clientOut bytes.Buffer

	logFile := filepath.Join(t.TempDir(), "agentrace.ndjson")
	p := New(Config{
		ClientReader: clientIn,
		ClientWriter: &clientOut,
		ServerReader: serverOut,
		ServerWriter: &serverIn,
		LogPath:      logFile,
		AgentID:      "cursor-parent",
		Server:       "sentrux",
		Now:          fixedTime,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = p.Run(ctx)

	raw, _ := os.ReadFile(logFile)
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatalf("expected log lines")
	}

	var ev Event
	if err := json.Unmarshal([]byte(lines[0]), &ev); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ev.EventType != "mcp_response" {
		t.Errorf("event_type = %q, want mcp_response", ev.EventType)
	}
	if !ev.Success {
		t.Errorf("success = false, want true")
	}
}

func TestProxy_LogsErrorResponse(t *testing.T) {
	t.Parallel()
	clientIn := strings.NewReader("")
	var serverIn bytes.Buffer
	serverOut := strings.NewReader(
		`{"jsonrpc":"2.0","id":42,"error":{"code":-32601,"message":"method not found"}}` + "\n")
	var clientOut bytes.Buffer

	logFile := filepath.Join(t.TempDir(), "agentrace.ndjson")
	p := New(Config{
		ClientReader: clientIn,
		ClientWriter: &clientOut,
		ServerReader: serverOut,
		ServerWriter: &serverIn,
		LogPath:      logFile,
		AgentID:      "cursor-parent",
		Server:       "sentrux",
		Now:          fixedTime,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = p.Run(ctx)

	raw, _ := os.ReadFile(logFile)
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatalf("expected log lines")
	}

	var ev Event
	if err := json.Unmarshal([]byte(lines[0]), &ev); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ev.Success {
		t.Errorf("success = true, want false")
	}
	if !strings.Contains(ev.ErrorMessage, "method not found") {
		t.Errorf("error_message = %q, want contains 'method not found'", ev.ErrorMessage)
	}
}

func TestProxy_HandlesNonJSON(t *testing.T) {
	t.Parallel()
	clientIn := strings.NewReader("garbage-line\n" + `{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n")
	var serverIn bytes.Buffer
	serverOut := strings.NewReader("")
	var clientOut bytes.Buffer

	logFile := filepath.Join(t.TempDir(), "agentrace.ndjson")
	p := New(Config{
		ClientReader: clientIn,
		ClientWriter: &clientOut,
		ServerReader: serverOut,
		ServerWriter: &serverIn,
		LogPath:      logFile,
		AgentID:      "cursor-parent",
		Server:       "sentrux",
		Now:          fixedTime,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = p.Run(ctx)

	if !strings.Contains(serverIn.String(), "ping") {
		t.Errorf("server did not receive valid JSON-RPC line")
	}
	if !strings.Contains(serverIn.String(), "garbage-line") {
		t.Errorf("server did not receive non-JSON line (should be forwarded verbatim)")
	}
}

func TestProxy_StopsOnContextCancel(t *testing.T) {
	t.Parallel()
	pr, pw := io.Pipe()
	defer pw.Close()

	var serverIn bytes.Buffer
	serverOut := strings.NewReader("")
	var clientOut bytes.Buffer

	p := New(Config{
		ClientReader: pr,
		ClientWriter: &clientOut,
		ServerReader: serverOut,
		ServerWriter: &serverIn,
		LogPath:      filepath.Join(t.TempDir(), "agentrace.ndjson"),
		AgentID:      "cursor-parent",
		Server:       "sentrux",
		Now:          time.Now,
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- p.Run(ctx) }()

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("Run did not exit on context cancel")
	}
}

func TestProxy_RaceSafeConcurrentLogs(t *testing.T) {
	t.Parallel()
	const n = 100
	var clientPayload bytes.Buffer
	var serverPayload bytes.Buffer
	for i := 0; i < n; i++ {
		clientPayload.WriteString(`{"jsonrpc":"2.0","id":` + intStr(i) + `,"method":"tools/call","params":{"name":"scan"}}` + "\n")
		serverPayload.WriteString(`{"jsonrpc":"2.0","id":` + intStr(i) + `,"result":{"ok":true}}` + "\n")
	}

	var serverIn bytes.Buffer
	var clientOut bytes.Buffer
	logFile := filepath.Join(t.TempDir(), "agentrace.ndjson")

	p := New(Config{
		ClientReader: &clientPayload,
		ClientWriter: &clientOut,
		ServerReader: &serverPayload,
		ServerWriter: &serverIn,
		LogPath:      logFile,
		AgentID:      "cursor-parent",
		Server:       "sentrux",
		Now:          time.Now,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = p.Run(ctx)

	raw, _ := os.ReadFile(logFile)
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 2*n {
		t.Errorf("got %d log lines, want %d", len(lines), 2*n)
	}
	for i, line := range lines {
		var ev Event
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Errorf("line %d: invalid json: %v", i, err)
			break
		}
	}
}

func TestLogger_ConcurrentWrites(t *testing.T) {
	t.Parallel()
	logFile := filepath.Join(t.TempDir(), "agentrace.ndjson")
	lg, err := newLogger(logFile)
	if err != nil {
		t.Fatalf("newLogger: %v", err)
	}
	defer lg.Close()

	const writers = 10
	const perWriter = 50
	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				_ = lg.write(Event{
					EventType: "mcp_request",
					Tool:      "test_tool",
					Server:    "test",
				})
			}
		}()
	}
	wg.Wait()
	_ = lg.Close()

	raw, _ := os.ReadFile(logFile)
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if got, want := len(lines), writers*perWriter; got != want {
		t.Errorf("got %d lines, want %d", got, want)
	}
}

func TestIDToString_Variants(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input json.RawMessage
		want  string
	}{
		{json.RawMessage(`42`), "42"},
		{json.RawMessage(`"abc-123"`), "abc-123"},
		{json.RawMessage(`null`), ""},
		{json.RawMessage(``), ""},
		{json.RawMessage(`3.14`), "3.14"},
	}
	for _, tt := range tests {
		got := idToString(tt.input)
		if got != tt.want {
			t.Errorf("idToString(%s) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func intStr(i int) string {
	const digits = "0123456789"
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = digits[i%10]
		i /= 10
	}
	return string(buf[pos:])
}
