package openclaw

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseGatewayBind_Loopback(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "openclaw.json")
	os.WriteFile(cfg, []byte(`{"gateway":{"bind":"loopback","port":18789}}`), 0600)

	result := ParseGatewayBind(cfg)
	if result != "loopback" {
		t.Errorf("expected loopback, got %s", result)
	}
}

func TestParseGatewayBind_IP(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "openclaw.json")
	os.WriteFile(cfg, []byte(`{"gateway":{"bind":"127.0.0.1"}}`), 0600)

	result := ParseGatewayBind(cfg)
	if result != "127.0.0.1" {
		t.Errorf("expected 127.0.0.1, got %s", result)
	}
}

func TestParseGatewayBind_MissingFile(t *testing.T) {
	result := ParseGatewayBind("/nonexistent/path.json")
	if result != "" {
		t.Errorf("expected empty string for missing file, got %s", result)
	}
}

func TestParseGatewayBind_ZeroAddr(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "openclaw.json")
	os.WriteFile(cfg, []byte(`{"gateway":{"bind":"0.0.0.0"}}`), 0600)

	result := ParseGatewayBind(cfg)
	if result != "0.0.0.0" {
		t.Errorf("expected 0.0.0.0, got %s", result)
	}
}

func TestIsLoopbackBind(t *testing.T) {
	tests := []struct {
		bind string
		want bool
	}{
		{"loopback", true},
		{"127.0.0.1", true},
		{"localhost", true},
		{"0.0.0.0", false},
		{"192.168.1.1", false},
		{"", false},
	}
	for _, tt := range tests {
		got := IsLoopbackBind(tt.bind)
		if got != tt.want {
			t.Errorf("IsLoopbackBind(%q) = %v, want %v", tt.bind, got, tt.want)
		}
	}
}

func TestCheckConfigPermissions_Correct(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "openclaw.json")
	os.WriteFile(cfg, []byte(`{}`), 0600)

	ok, perms := CheckConfigPermissions(cfg)
	if !ok {
		t.Errorf("expected ok=true for 0600, got perms=%s", perms)
	}
}

func TestCheckConfigPermissions_TooOpen(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "openclaw.json")
	os.WriteFile(cfg, []byte(`{}`), 0644)

	ok, perms := CheckConfigPermissions(cfg)
	if ok {
		t.Errorf("expected ok=false for 0644, got perms=%s", perms)
	}
}

func TestCheckConfigPermissions_Missing(t *testing.T) {
	ok, _ := CheckConfigPermissions("/nonexistent/path.json")
	if ok {
		t.Error("expected ok=false for missing file")
	}
}

func TestCheckHardcodedKeys_Clean(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "openclaw.json")
	os.WriteFile(cfg, []byte(`{"gateway":{"auth":{"token":"${OPENCLAW_TOKEN}"}}}`), 0600)

	found := CheckHardcodedKeys(cfg)
	if len(found) != 0 {
		t.Errorf("expected no hardcoded keys, found %v", found)
	}
}

func TestCheckHardcodedKeys_Dirty(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "openclaw.json")
	os.WriteFile(cfg, []byte(`{"api_key":"sk-ant-abc123def"}`), 0600)

	found := CheckHardcodedKeys(cfg)
	if len(found) == 0 {
		t.Error("expected hardcoded key detection, got none")
	}
}

func TestCheckEvolveConstraint_Disabled(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	os.WriteFile(envFile, []byte("EVOLVE_ALLOW_SELF_MODIFY=false\n"), 0600)

	ok := CheckEvolveConstraint(dir)
	if !ok {
		t.Error("expected ok=true when self-modify is false")
	}
}

func TestCheckEvolveConstraint_Enabled(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	os.WriteFile(envFile, []byte("EVOLVE_ALLOW_SELF_MODIFY=true\n"), 0600)

	ok := CheckEvolveConstraint(dir)
	if ok {
		t.Error("expected ok=false when self-modify is true")
	}
}

func TestCheckEvolveConstraint_Missing(t *testing.T) {
	dir := t.TempDir()
	ok := CheckEvolveConstraint(dir)
	if !ok {
		t.Error("expected ok=true when .env is missing (defaults to false)")
	}
}

func TestRunAudit_AllPass(t *testing.T) {
	dir := t.TempDir()

	cfg := filepath.Join(dir, "openclaw.json")
	os.WriteFile(cfg, []byte(`{"gateway":{"bind":"loopback","port":18789}}`), 0600)

	logDir := filepath.Join(dir, "logs")
	os.MkdirAll(logDir, 0755)

	envFile := filepath.Join(dir, ".env")
	os.WriteFile(envFile, []byte("EVOLVE_ALLOW_SELF_MODIFY=false\n"), 0600)

	results := RunAudit(dir)
	for _, r := range results {
		if !r.Pass {
			t.Errorf("expected all pass, got FAIL: %s", r.Label)
		}
	}
}

func TestRunAudit_FailOnBind(t *testing.T) {
	dir := t.TempDir()

	cfg := filepath.Join(dir, "openclaw.json")
	os.WriteFile(cfg, []byte(`{"gateway":{"bind":"0.0.0.0"}}`), 0600)

	logDir := filepath.Join(dir, "logs")
	os.MkdirAll(logDir, 0755)

	results := RunAudit(dir)
	foundBindFail := false
	for _, r := range results {
		if !r.Pass && r.Label == "Network isolation (loopback)" {
			foundBindFail = true
		}
	}
	if !foundBindFail {
		t.Error("expected bind check to fail for 0.0.0.0")
	}
}

func TestCheckDeadlockSignatures_Clean(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "gateway.err.log")
	os.WriteFile(logFile, []byte("2026-03-09 INFO: all good\n2026-03-09 INFO: normal operation\n"), 0644)

	found := CheckDeadlockSignatures(logFile, 200)
	if found {
		t.Error("expected no deadlock signatures in clean log")
	}
}

func TestCheckDeadlockSignatures_Timeout(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "gateway.err.log")
	os.WriteFile(logFile, []byte("2026-03-09 ERROR: request timeout after 30s\n"), 0644)

	found := CheckDeadlockSignatures(logFile, 200)
	if !found {
		t.Error("expected deadlock signature for 'timeout'")
	}
}

func TestCheckDeadlockSignatures_408(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "gateway.err.log")
	os.WriteFile(logFile, []byte("2026-03-09 ERROR: received 408 from upstream\n"), 0644)

	found := CheckDeadlockSignatures(logFile, 200)
	if !found {
		t.Error("expected deadlock signature for '408'")
	}
}

func TestCheckDeadlockSignatures_MissingFile(t *testing.T) {
	found := CheckDeadlockSignatures("/nonexistent/log.log", 200)
	if found {
		t.Error("expected false for missing file")
	}
}
