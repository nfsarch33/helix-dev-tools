package openclaw

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type gatewayConfig struct {
	Gateway struct {
		Bind string `json:"bind"`
		Port int    `json:"port"`
	} `json:"gateway"`
}

// ParseGatewayBind reads the OpenClaw config and returns the gateway.bind value.
func ParseGatewayBind(configPath string) string {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}
	var cfg gatewayConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ""
	}
	return cfg.Gateway.Bind
}

// IsLoopbackBind returns true if the bind value is safe (loopback/localhost/127.0.0.1).
func IsLoopbackBind(bind string) bool {
	switch strings.ToLower(bind) {
	case "loopback", "127.0.0.1", "localhost":
		return true
	}
	return false
}

// CheckConfigPermissions verifies the config file has 0600 permissions.
func CheckConfigPermissions(configPath string) (ok bool, perms string) {
	info, err := os.Stat(configPath)
	if err != nil {
		return false, "missing"
	}
	mode := info.Mode().Perm()
	perms = fmt.Sprintf("%04o", mode)
	return mode == 0600, perms
}

var hardcodedKeyPrefixes = []string{
	"sk-ant-",
	"sk-proj-",
	"sk-or-",
	"gsk_",
	"xai-",
}

// CheckHardcodedKeys scans the config file for hardcoded API key patterns.
func CheckHardcodedKeys(configPath string) []string {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}
	content := string(data)
	var found []string
	for _, prefix := range hardcodedKeyPrefixes {
		if strings.Contains(content, prefix) {
			found = append(found, prefix+"*")
		}
	}
	return found
}

// CheckEvolveConstraint verifies EVOLVE_ALLOW_SELF_MODIFY is not true.
// Reads from <openclawDir>/.env. Missing file defaults to safe (false).
func CheckEvolveConstraint(openclawDir string) bool {
	envFile := filepath.Join(openclawDir, ".env")
	f, err := os.Open(envFile)
	if err != nil {
		return true // missing .env = safe default
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "EVOLVE_ALLOW_SELF_MODIFY=") {
			val := strings.TrimPrefix(line, "EVOLVE_ALLOW_SELF_MODIFY=")
			val = strings.Trim(val, `"' `)
			return strings.ToLower(val) != "true"
		}
	}
	return true // not found = safe default
}

var deadlockPatterns = []string{
	"lane wait exceeded",
	"408",
	"timeout",
	"etimedout",
}

// CheckDeadlockSignatures scans the last N lines of the gateway log for
// cognitive deadlock or API timeout signatures.
func CheckDeadlockSignatures(logPath string, tailLines int) bool {
	f, err := os.Open(logPath)
	if err != nil {
		return false
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	start := 0
	if len(lines) > tailLines {
		start = len(lines) - tailLines
	}

	for _, line := range lines[start:] {
		lower := strings.ToLower(line)
		for _, pat := range deadlockPatterns {
			if strings.Contains(lower, pat) {
				return true
			}
		}
	}
	return false
}

// AuditResult represents a single security check result.
type AuditResult struct {
	Label  string
	Pass   bool
	Detail string
}

// RunAudit performs a full security red-line audit on the OpenClaw directory.
func RunAudit(openclawDir string) []AuditResult {
	var results []AuditResult
	configPath := filepath.Join(openclawDir, "openclaw.json")
	logDir := filepath.Join(openclawDir, "logs")

	// 1. Loopback binding
	bind := ParseGatewayBind(configPath)
	if bind == "" {
		results = append(results, AuditResult{"Config file readable", false, configPath})
	} else if IsLoopbackBind(bind) {
		results = append(results, AuditResult{"Network isolation (loopback)", true, bind})
	} else {
		results = append(results, AuditResult{"Network isolation (loopback)", false, "bind=" + bind})
	}

	// 2. Log directory exists
	if info, err := os.Stat(logDir); err != nil || !info.IsDir() {
		results = append(results, AuditResult{"Log directory exists", false, logDir})
	} else {
		results = append(results, AuditResult{"Log directory exists", true, logDir})
	}

	// 3. Config permissions (600)
	ok, perms := CheckConfigPermissions(configPath)
	if ok {
		results = append(results, AuditResult{"Config permissions (600)", true, perms})
	} else {
		results = append(results, AuditResult{"Config permissions (600)", false, "current=" + perms})
	}

	// 4. Evolutionary constraint
	if CheckEvolveConstraint(openclawDir) {
		results = append(results, AuditResult{"Evolutionary constraint (self-modify disabled)", true, ""})
	} else {
		results = append(results, AuditResult{"Evolutionary constraint (self-modify disabled)", false, "EVOLVE_ALLOW_SELF_MODIFY=true"})
	}

	// 5. No hardcoded API keys
	keys := CheckHardcodedKeys(configPath)
	if len(keys) == 0 {
		results = append(results, AuditResult{"No hardcoded API keys", true, ""})
	} else {
		results = append(results, AuditResult{"No hardcoded API keys", false, strings.Join(keys, ", ")})
	}

	return results
}
