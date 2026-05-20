package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendEvent_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")

	event := map[string]any{
		"type":      "PreToolUse",
		"timestamp": 1715300000000,
		"tool_name": "Shell",
	}
	raw, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}

	if err := AppendEvent(path, raw); err != nil {
		t.Fatalf("AppendEvent failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &decoded); err != nil {
		t.Fatalf("line is not valid JSON: %v", err)
	}
	if decoded["tool_name"] != "Shell" {
		t.Errorf("expected tool_name=Shell, got %v", decoded["tool_name"])
	}
}

func TestAppendEvent_ValidatesJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")

	err := AppendEvent(path, []byte("not-json{{{"))
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestAppendEvent_MultipleAppends(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")

	for i := 0; i < 3; i++ {
		raw, _ := json.Marshal(map[string]int{"n": i})
		if err := AppendEvent(path, raw); err != nil {
			t.Fatalf("append %d failed: %v", i, err)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
}

func TestAppendEvent_CreatesFileIfMissing(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "sub", "events.jsonl")

	raw, _ := json.Marshal(map[string]string{"type": "Stop"})
	if err := AppendEvent(nested, raw); err != nil {
		t.Fatalf("expected auto-create to succeed: %v", err)
	}

	if _, err := os.Stat(nested); os.IsNotExist(err) {
		t.Fatal("file was not created")
	}
}

func TestRotateEvents_RotatesOversizeFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")
	original := strings.Repeat(`{"type":"PreToolUse"}`+"\n", 8)
	if err := os.WriteFile(path, []byte(original), filePerms); err != nil {
		t.Fatal(err)
	}

	result, err := RotateEvents(path, RotationOptions{MaxBytes: 32, MaxArchives: 5})
	if err != nil {
		t.Fatalf("RotateEvents failed: %v", err)
	}

	if !result.Rotated {
		t.Fatal("expected rotation to occur")
	}
	if result.ArchivePath == "" {
		t.Fatal("expected archive path to be returned")
	}
	archived, err := os.ReadFile(result.ArchivePath)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	if string(archived) != original {
		t.Fatalf("archive mismatch\ngot: %q\nwant: %q", string(archived), original)
	}
	active, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read active file: %v", err)
	}
	if len(active) != 0 {
		t.Fatalf("active events file should be empty after rotation, got %d bytes", len(active))
	}
}

func TestRotateEvents_SkipsSmallFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")
	original := `{"type":"Stop"}` + "\n"
	if err := os.WriteFile(path, []byte(original), filePerms); err != nil {
		t.Fatal(err)
	}

	result, err := RotateEvents(path, RotationOptions{MaxBytes: 1024, MaxArchives: 5})
	if err != nil {
		t.Fatalf("RotateEvents failed: %v", err)
	}
	if result.Rotated {
		t.Fatal("small file should not rotate")
	}
	active, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read active file: %v", err)
	}
	if string(active) != original {
		t.Fatalf("active file changed\ngot: %q\nwant: %q", string(active), original)
	}
}

func TestRotateEvents_PrunesOldArchives(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")
	for _, suffix := range []string{
		"20260101T000000.000000000Z",
		"20260102T000000.000000000Z",
		"20260103T000000.000000000Z",
	} {
		if err := os.WriteFile(path+"."+suffix, []byte(suffix), filePerms); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(path, []byte(strings.Repeat(`{"type":"Notification"}`+"\n", 8)), filePerms); err != nil {
		t.Fatal(err)
	}

	result, err := RotateEvents(path, RotationOptions{MaxBytes: 32, MaxArchives: 2})
	if err != nil {
		t.Fatalf("RotateEvents failed: %v", err)
	}
	if !result.Rotated {
		t.Fatal("expected rotation to occur")
	}
	matches, err := filepath.Glob(path + ".*")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) != 2 {
		t.Fatalf("expected exactly 2 retained archives, got %d: %v", len(matches), matches)
	}
	for _, match := range matches {
		if strings.Contains(match, "20260101") || strings.Contains(match, "20260102") {
			t.Fatalf("old archive was not pruned: %s", match)
		}
	}
}

func TestBuildEvent_MapsSessionStartAlias(t *testing.T) {
	raw, err := BuildEvent(EventInput{
		EventName: "session_start",
		Timestamp: 1715300000000,
		SessionID: "cursor-tools__v5009r-1",
		AgentID:   "root",
		Payload: map[string]any{
			"sprint_id": "v5009r",
			"story_id":  "v5009r-1",
			"repo":      "cursor-tools",
		},
	})
	if err != nil {
		t.Fatalf("BuildEvent returned error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := decoded["type"]; got != "UserPromptSubmit" {
		t.Fatalf("type = %v, want UserPromptSubmit", got)
	}
	payload, ok := decoded["payload"].(map[string]any)
	if !ok {
		t.Fatalf("payload missing or wrong type: %#v", decoded["payload"])
	}
	if payload["story_id"] != "v5009r-1" {
		t.Fatalf("payload.story_id = %v, want v5009r-1", payload["story_id"])
	}
}

func TestBuildEvent_MapsShellEndFailureAlias(t *testing.T) {
	raw, err := BuildEvent(EventInput{
		EventName:  "shell_end",
		Timestamp:  1715300000200,
		SessionID:  "cursor-tools__v5009r-1",
		AgentID:    "root",
		ToolCallID: "tool-1",
		ToolName:   "Shell",
		ExitCode:   17,
	})
	if err != nil {
		t.Fatalf("BuildEvent returned error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := decoded["type"]; got != "PostToolUseFailure" {
		t.Fatalf("type = %v, want PostToolUseFailure", got)
	}
	payload, ok := decoded["payload"].(map[string]any)
	if !ok {
		t.Fatalf("payload missing or wrong type: %#v", decoded["payload"])
	}
	if payload["exit_code"] != float64(17) {
		t.Fatalf("payload.exit_code = %v, want 17", payload["exit_code"])
	}
}
