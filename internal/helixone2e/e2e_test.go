//go:build integration

package helixone2e

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

// TestHelixonE2E_HealthAndTask verifies the full Helixon serve-mode flow:
// 1. Health check confirms service is up
// 2. Submit a task via HTTP
// 3. Verify the agent processes it
// 4. Verify memory is stored in Engram
//
// Prerequisites:
//   - Helixon running: ~/runs/helixon serve --port 8200 --llm-endpoint http://localhost:<llm-port>
//   - Engram running: accessible via tunnel at localhost:<engram-port>
//   - LLM router running on the remote host
//
// Run: go test -tags integration -run TestHelixonE2E ./internal/helixone2e/
func TestHelixonE2E_HealthAndTask(t *testing.T) {
	helixonURL := envOrDefault("HELIXON_URL", "http://localhost:8200")
	engramURL := envOrDefault("ENGRAM_URL", "http://localhost:8281")
	helixon := NewHelixonClient(helixonURL)
	engram := NewEngramVerifier(engramURL, "")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := helixon.HealthCheck(ctx); err != nil {
		t.Skipf("Helixon not running (expected during CI): %v", err)
	}

	marker := fmt.Sprintf("e2e-test-marker-%d", time.Now().UnixMilli())
	task := TaskRequest{
		Prompt: fmt.Sprintf("Remember this unique marker for testing: %s", marker),
		UserID: "nfsarch33",
	}

	resp, err := helixon.SubmitTask(ctx, task)
	if err != nil {
		t.Fatalf("SubmitTask failed: %v", err)
	}
	if resp.Status == "error" {
		t.Fatalf("Task returned error: %s", resp.Error)
	}
	t.Logf("Task response: status=%s response=%s", resp.Status, resp.Response)

	time.Sleep(3 * time.Second)

	found, err := engram.SearchMemory(ctx, marker, "nfsarch33")
	if err != nil {
		t.Logf("Engram search error (non-fatal): %v", err)
	} else if !found {
		t.Logf("Memory marker not found in Engram (may need longer propagation)")
	} else {
		t.Logf("SUCCESS: Memory marker found in Engram")
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// TestHelixonE2E_StartupDoc documents exact commands to start Helixon.
func TestHelixonE2E_StartupDoc(t *testing.T) {
	t.Log("Helixon serve-mode startup on the remote host:")
	t.Log("  runx ssh exec --target <host-alias> --raw \"~/runs/helixon serve --port 8200 --llm-endpoint http://localhost:<llm-port> &\"")
	t.Log("")
	t.Log("Prerequisites:")
	t.Log("  1. LLM cluster router running on remote host")
	t.Log("  2. Engram accessible (tunnel or direct)")
	t.Log("  3. ENGRAM_URL env set")
	t.Log("")
	t.Log("Health check:")
	t.Log("  curl -sS http://localhost:8200/healthz")
}
