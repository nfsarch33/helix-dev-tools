package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/nfsarch33/cursor-tools/internal/hookio"
)

func TestRtkRewriteHandler_NoRtk(t *testing.T) {
	h := &rtkRewriteHandler{rtkBin: "/nonexistent/rtk"}
	input := &hookio.Input{Command: "git status"}
	resp, err := h.handle(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != nil {
		t.Fatalf("expected nil response when rtk binary missing, got %+v", resp)
	}
}

func TestRtkRewriteHandler_EmptyCommand(t *testing.T) {
	h := &rtkRewriteHandler{rtkBin: "rtk"}
	input := &hookio.Input{Command: ""}
	resp, err := h.handle(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != nil {
		t.Fatalf("expected nil for empty command, got %+v", resp)
	}
}

func TestRtkRewriteHandler_ParsesCursorInput(t *testing.T) {
	payload := `{"tool_name":"Bash","command":"git status"}`
	input, err := hookio.ReadInput(strings.NewReader(payload))
	if err != nil {
		t.Fatalf("failed to parse input: %v", err)
	}
	if input.ToolName != "Bash" {
		t.Errorf("expected ToolName=Bash, got %q", input.ToolName)
	}
	if input.Command != "git status" {
		t.Errorf("expected Command='git status', got %q", input.Command)
	}
}

func TestRtkRewriteHandler_OutputFormat(t *testing.T) {
	resp := rtkRewriteResponse("ls -la", "rtk ls -la")
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	if !bytes.Contains(data, []byte("hookSpecificOutput")) {
		t.Errorf("response missing hookSpecificOutput, got: %s", data)
	}
	if !bytes.Contains(data, []byte("rtk ls -la")) {
		t.Errorf("response missing rewritten command, got: %s", data)
	}
}

func TestRtkRewriteRoundTrip(t *testing.T) {
	rtkBin, err := rtkBinPath()
	if err != nil {
		t.Skipf("rtk not found: %v", err)
	}

	h := &rtkRewriteHandler{rtkBin: rtkBin}
	input := &hookio.Input{Command: "git status"}
	resp, err := h.handle(input)
	if err != nil {
		t.Fatalf("handle error: %v", err)
	}
	// rtk may or may not rewrite "git status" — either nil (no change) or a valid response is fine
	if resp != nil {
		data, _ := json.Marshal(resp)
		if !bytes.Contains(data, []byte("hookSpecificOutput")) {
			t.Errorf("expected hookSpecificOutput in rewrite response, got: %s", data)
		}
	}
}

func TestRunRtkRewrite_EmptyStdin(t *testing.T) {
	var stdout bytes.Buffer
	stdin := strings.NewReader("")
	err := runRtkRewriteIO(stdin, &stdout)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// empty stdin => no output (passthrough)
	if stdout.Len() > 0 {
		t.Logf("stdout: %s", stdout.String())
	}
}

func TestRtkVersionCheck(t *testing.T) {
	ok := checkRtkVersion("/nonexistent/rtk")
	if ok {
		t.Error("expected false for nonexistent binary")
	}
}
