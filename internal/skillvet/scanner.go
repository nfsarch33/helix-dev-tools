package skillvet

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Finding struct {
	Severity    string `json:"severity"`
	Description string `json:"description"`
	File        string `json:"file"`
}

type Result struct {
	Skill     string    `json:"skill"`
	Criticals int       `json:"criticals"`
	Warnings  int       `json:"warnings"`
	Status    string    `json:"status"`
	Findings  []Finding `json:"findings,omitempty"`
}

func (r Result) JSON() (string, error) {
	b, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

type pattern struct {
	re   *regexp.Regexp
	desc string
}

var criticalPatterns = []pattern{
	{regexp.MustCompile(`webhook\.site|ngrok\.io|requestbin|pipedream\.net`), "Exfiltration endpoint"},
	{regexp.MustCompile(`printenv|env\s*\||\$\{!|set\s*\|`), "Env variable harvesting"},
	{regexp.MustCompile(`ANTHROPIC_API_KEY|OPENAI_API_KEY|TELEGRAM_BOT_TOKEN|STRIPE_SECRET`), "Foreign credential access"},
	{regexp.MustCompile(`base64\s+(--decode|-d|-D)|atob\(|Buffer\.from`), "Code obfuscation (base64)"},
	{regexp.MustCompile(`\.\./\.\./|~/\.ssh|~/\.aws|~/\.gnupg`), "Path traversal / sensitive dirs"},
	{regexp.MustCompile(`curl.*--data.*\$|wget.*--post`), "Data exfiltration via curl/wget"},
	{regexp.MustCompile(`/dev/tcp/|nc\s+-e|socat.*exec|ncat.*-e`), "Reverse/bind shell"},
	{regexp.MustCompile(`dotenv|source\s+\.env|cat\s+\.env`), "Dotenv file theft"},
	{regexp.MustCompile(`ignore previous instructions|disregard all|override system prompt`), "Prompt injection"},
	{regexp.MustCompile(`send.*secret|email.*password|exfiltrate`), "LLM tool exploitation"},
	{regexp.MustCompile(`write.*AGENTS\.md|modify.*SOUL\.md|overwrite.*clawdbot`), "Agent config tampering"},
	{regexp.MustCompile(`curl.*\|.*bash|wget.*\|.*sh`), "Suspicious setup (curl pipe bash)"},
	{regexp.MustCompile(`download.*binary|install.*executable|paste.*terminal`), "Social engineering download"},
	{regexp.MustCompile(`curl.*http.*\|.*bash|curl.*http.*\|.*sh`), "Pipe-to-shell (HTTP)"},
	{regexp.MustCompile(`fromCharCode|getattr.*__import__|eval.*compile`), "String construction evasion"},
	{regexp.MustCompile(`Date\.now\(\).*>|setTimeout.*86400`), "Time bomb detection"},
	{regexp.MustCompile(`base64.*-[dD].*\|.*bash`), "Base64 pipe-to-interpreter"},
	{regexp.MustCompile(`bash.*-i.*>/dev/tcp`), "Bash reverse shell"},
	{regexp.MustCompile(`socket\.connect.*pty\.spawn|dup2.*subprocess`), "Python reverse shell"},
	{regexp.MustCompile(`cat.*\.pem|cat.*\.aws/credentials|cat.*id_rsa`), "Credential file access"},
}

var warningPatterns = []pattern{
	{regexp.MustCompile(`child_process|execSync|subprocess\.run|os\.system`), "Subprocess execution"},
	{regexp.MustCompile(`import.*requests|import.*axios|fetch\(`), "Network requests"},
	{regexp.MustCompile(`writeFile|open.*'w'|fs\.append`), "Filesystem writes"},
	{regexp.MustCompile(`docker.*pull.*[^/]*\.[^/]*/`), "Docker untrusted registry"},
	{regexp.MustCompile(`curl.*-k|verify=False|NODE_TLS_REJECT`), "Insecure transport"},
}

// ScanSkill scans a skill directory for security issues.
func ScanSkill(skillPath string) (Result, error) {
	info, err := os.Stat(skillPath)
	if err != nil {
		return Result{}, fmt.Errorf("cannot access skill path: %w", err)
	}
	if !info.IsDir() {
		return Result{}, fmt.Errorf("skill path is not a directory: %s", skillPath)
	}

	skillName := filepath.Base(skillPath)
	result := Result{Skill: skillName}

	err = filepath.Walk(skillPath, func(path string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if fi.IsDir() {
			return nil
		}
		relPath, _ := filepath.Rel(skillPath, path)
		data, readErr := os.ReadFile(path) // #nosec G122 -- read-only scan callback; TOCTOU risk accepted for security audit
		if readErr != nil {
			return nil
		}
		content := string(data)
		for _, line := range strings.Split(content, "\n") {
			for _, p := range criticalPatterns {
				if p.re.MatchString(line) {
					result.Criticals++
					result.Findings = append(result.Findings, Finding{
						Severity:    "CRITICAL",
						Description: p.desc,
						File:        relPath,
					})
				}
			}
			for _, p := range warningPatterns {
				if p.re.MatchString(line) {
					result.Warnings++
					result.Findings = append(result.Findings, Finding{
						Severity:    "WARNING",
						Description: p.desc,
						File:        relPath,
					})
				}
			}
		}
		return nil
	})
	if err != nil {
		return Result{}, fmt.Errorf("scanning skill: %w", err)
	}

	result.Status = "clean"
	if result.Warnings > 0 {
		result.Status = "warning"
	}
	if result.Criticals > 0 {
		result.Status = "critical"
	}

	return result, nil
}
