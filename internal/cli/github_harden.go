package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"
)

// hardenPolicy holds overridable settings loaded from the config file.
type hardenPolicy struct {
	DefaultBranch       string `yaml:"default_branch"`
	RequireReviews      bool   `yaml:"require_reviews"`
	RequiredApprovals   int    `yaml:"required_approvals"`
	DismissStaleReviews bool   `yaml:"dismiss_stale_reviews"`
	TagPattern          string `yaml:"tag_pattern"`
	CodeownersContent   string `yaml:"codeowners_content"`
}

func defaultHardenPolicy() hardenPolicy {
	return hardenPolicy{
		DefaultBranch:       "main",
		RequireReviews:      false,
		RequiredApprovals:   1,
		DismissStaleReviews: true,
		TagPattern:          "v*",
		CodeownersContent:   "* @nfsarch33\n",
	}
}

type hardenStepResult struct {
	Step    string `json:"step"`
	Status string `json:"status"` // OK, WARN, SKIP, FAIL
	Detail string `json:"detail,omitempty"`
}

type hardenReport struct {
	Repo    string             `json:"repo"`
	Results []hardenStepResult `json:"results"`
}

var (
	hardenRepo           string
	hardenAll            bool
	hardenDryRun         bool
	hardenRequireReviews bool
	hardenConfigPath     string
	hardenOutputJSON     bool
)

var githubCmd = &cobra.Command{
	Use:   "github",
	Short: "GitHub repository management commands",
}

var githubHardenCmd = &cobra.Command{
	Use:   "harden",
	Short: "Apply security and governance hardening to GitHub repositories",
	Long: "Applies 10 hardening steps: merge strategy, Dependabot, secret scanning,\n" +
		"CodeQL, Actions token permissions, vulnerability reporting, CODEOWNERS,\n" +
		"branch protection, and tag protection.",
	SilenceUsage: true,
	RunE:         runGithubHarden,
}

func init() {
	githubHardenCmd.Flags().StringVar(&hardenRepo, "repo", "", "Repository name or runx alias (owner/repo)")
	githubHardenCmd.Flags().BoolVar(&hardenAll, "all", false, "Target all owned repositories")
	githubHardenCmd.Flags().BoolVar(&hardenDryRun, "dry-run", false, "Preview changes without applying")
	githubHardenCmd.Flags().BoolVar(&hardenRequireReviews, "require-reviews", false, "Enable PR approval requirements (default off for solo owner)")
	githubHardenCmd.Flags().StringVar(&hardenConfigPath, "config", "", "Path to policy config YAML")
	githubHardenCmd.Flags().BoolVar(&hardenOutputJSON, "json", false, "Output results as JSON")
	githubCmd.AddCommand(githubHardenCmd)
}

// ghAPIRunner abstracts gh api calls for testability.
type ghAPIRunner func(ctx context.Context, method, endpoint string, body string) ([]byte, error)

var defaultGHAPI ghAPIRunner = func(ctx context.Context, method, endpoint string, body string) ([]byte, error) {
	args := []string{"api", endpoint, "--method", method}
	if body != "" {
		args = append(args, "--input", "-")
	}
	cmd := exec.CommandContext(ctx, "gh", args...)
	if body != "" {
		cmd.Stdin = strings.NewReader(body)
	}
	cmd.Stderr = os.Stderr
	return cmd.Output()
}

// ghGraphQLRunner abstracts gh api graphql calls.
type ghGraphQLRunner func(ctx context.Context, query string) ([]byte, error)

var defaultGHGraphQL ghGraphQLRunner = func(ctx context.Context, query string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", "api", "graphql", "--input", "-")
	cmd.Stdin = strings.NewReader(query)
	cmd.Stderr = os.Stderr
	return cmd.Output()
}

func runGithubHarden(cmd *cobra.Command, _ []string) error {
	if !hardenAll && hardenRepo == "" {
		return fmt.Errorf("specify --repo <owner/repo> or --all")
	}
	policy := loadHardenPolicy(hardenConfigPath)
	if hardenRequireReviews {
		policy.RequireReviews = true
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
	defer cancel()

	repos, err := resolveHardenRepos(ctx, hardenRepo, hardenAll)
	if err != nil {
		return err
	}

	var reports []hardenReport
	for _, repo := range repos {
		report := hardenOneRepo(ctx, cmd.OutOrStdout(), repo, policy)
		reports = append(reports, report)
	}

	if hardenOutputJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(reports)
	}
	return nil
}

func loadHardenPolicy(path string) hardenPolicy {
	policy := defaultHardenPolicy()
	if path == "" {
		home := os.Getenv("HOME")
		path = filepath.Join(home, ".config", "cursor-tools", "github-harden.yaml")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return policy
	}
	_ = yaml.Unmarshal(data, &policy)
	return policy
}

func resolveHardenRepos(ctx context.Context, single string, all bool) ([]string, error) {
	if !all {
		return []string{single}, nil
	}
	out, err := defaultGHAPI(ctx, "GET", "/user/repos?per_page=100&affiliation=owner&type=owner", "")
	if err != nil {
		return nil, fmt.Errorf("listing repos: %w", err)
	}
	var repos []struct {
		FullName string `json:"full_name"`
		Archived bool   `json:"archived"`
		Fork     bool   `json:"fork"`
	}
	if err := json.Unmarshal(out, &repos); err != nil {
		return nil, fmt.Errorf("parsing repos: %w", err)
	}
	var names []string
	for _, r := range repos {
		if r.Archived || r.Fork {
			continue
		}
		names = append(names, r.FullName)
	}
	return names, nil
}

func hardenOneRepo(ctx context.Context, w io.Writer, repo string, policy hardenPolicy) hardenReport {
	report := hardenReport{Repo: repo}
	if !hardenOutputJSON {
		fmt.Fprintf(w, "\n=== %s ===\n", repo)
	}

	steps := []struct {
		name string
		fn   func(ctx context.Context, repo string, policy hardenPolicy) hardenStepResult
	}{
		{"merge-strategy", stepMergeStrategy},
		{"dependabot-alerts", stepDependabotAlerts},
		{"dependabot-security-fixes", stepDependabotSecurityFixes},
		{"secret-scanning", stepSecretScanning},
		{"codeql", stepCodeQL},
		{"actions-token", stepActionsToken},
		{"vulnerability-reporting", stepVulnerabilityReporting},
		{"codeowners", stepCodeowners},
		{"branch-protection", stepBranchProtection},
		{"tag-protection", stepTagProtection},
	}

	for _, s := range steps {
		result := s.fn(ctx, repo, policy)
		report.Results = append(report.Results, result)
		if !hardenOutputJSON {
			printStepResult(w, result)
		}
	}
	return report
}

func printStepResult(w io.Writer, r hardenStepResult) {
	status := r.Status
	detail := ""
	if r.Detail != "" {
		detail = " — " + r.Detail
	}
	fmt.Fprintf(w, "  [%4s] %s%s\n", status, r.Step, detail)
}

func result(step, status, detail string) hardenStepResult {
	return hardenStepResult{Step: step, Status: status, Detail: detail}
}

func stepMergeStrategy(ctx context.Context, repo string, _ hardenPolicy) hardenStepResult {
	step := "merge-strategy"
	if hardenDryRun {
		return result(step, "SKIP", "dry-run")
	}
	body := `{"allow_squash_merge":true,"allow_rebase_merge":true,"allow_merge_commit":false,"allow_auto_merge":true,"delete_branch_on_merge":true,"squash_merge_commit_title":"PR_TITLE","squash_merge_commit_message":"BLANK"}`
	_, err := defaultGHAPI(ctx, "PATCH", "/repos/"+repo, body)
	if err != nil {
		return result(step, "FAIL", err.Error())
	}
	return result(step, "OK", "squash+rebase, auto-merge, delete-on-merge")
}

func stepDependabotAlerts(ctx context.Context, repo string, _ hardenPolicy) hardenStepResult {
	step := "dependabot-alerts"
	if hardenDryRun {
		return result(step, "SKIP", "dry-run")
	}
	_, err := defaultGHAPI(ctx, "PUT", "/repos/"+repo+"/vulnerability-alerts", "")
	if err != nil {
		return result(step, "WARN", err.Error())
	}
	return result(step, "OK", "enabled")
}

func stepDependabotSecurityFixes(ctx context.Context, repo string, _ hardenPolicy) hardenStepResult {
	step := "dependabot-security-fixes"
	if hardenDryRun {
		return result(step, "SKIP", "dry-run")
	}
	_, err := defaultGHAPI(ctx, "PUT", "/repos/"+repo+"/automated-security-fixes", "")
	if err != nil {
		return result(step, "WARN", err.Error())
	}
	return result(step, "OK", "enabled")
}

func stepSecretScanning(ctx context.Context, repo string, _ hardenPolicy) hardenStepResult {
	step := "secret-scanning"
	if hardenDryRun {
		return result(step, "SKIP", "dry-run")
	}
	body := `{"security_and_analysis":{"secret_scanning":{"status":"enabled"},"secret_scanning_push_protection":{"status":"enabled"}}}`
	_, err := defaultGHAPI(ctx, "PATCH", "/repos/"+repo, body)
	if err != nil {
		return result(step, "WARN", err.Error())
	}
	return result(step, "OK", "scanning + push protection")
}

func stepCodeQL(ctx context.Context, repo string, _ hardenPolicy) hardenStepResult {
	step := "codeql"
	if hardenDryRun {
		return result(step, "SKIP", "dry-run")
	}
	body := `{"state":"configured","query_suite":"default"}`
	_, err := defaultGHAPI(ctx, "PATCH", "/repos/"+repo+"/code-scanning/default-setup", body)
	if err != nil {
		return result(step, "WARN", err.Error())
	}
	return result(step, "OK", "default setup enabled")
}

func stepActionsToken(ctx context.Context, repo string, _ hardenPolicy) hardenStepResult {
	step := "actions-token"
	if hardenDryRun {
		return result(step, "SKIP", "dry-run")
	}
	body := `{"default_workflow_permissions":"read","can_approve_pull_request_reviews":false}`
	_, err := defaultGHAPI(ctx, "PUT", "/repos/"+repo+"/actions/permissions/workflow", body)
	if err != nil {
		return result(step, "WARN", err.Error())
	}
	return result(step, "OK", "read-only default token")
}

func stepVulnerabilityReporting(ctx context.Context, repo string, _ hardenPolicy) hardenStepResult {
	step := "vulnerability-reporting"
	if hardenDryRun {
		return result(step, "SKIP", "dry-run")
	}
	_, err := defaultGHAPI(ctx, "PUT", "/repos/"+repo+"/private-vulnerability-reporting", "")
	if err != nil {
		return result(step, "WARN", err.Error())
	}
	return result(step, "OK", "private vulnerability reporting enabled")
}

func stepCodeowners(ctx context.Context, repo string, policy hardenPolicy) hardenStepResult {
	step := "codeowners"
	if hardenDryRun {
		return result(step, "SKIP", "dry-run")
	}
	// Check if CODEOWNERS already exists
	_, err := defaultGHAPI(ctx, "GET", "/repos/"+repo+"/contents/.github/CODEOWNERS", "")
	if err == nil {
		return result(step, "OK", "already exists")
	}
	content := policy.CodeownersContent
	if content == "" {
		content = "* @nfsarch33\n"
	}
	encoded := encodeBase64([]byte(content))
	body := fmt.Sprintf(`{"message":"chore: add CODEOWNERS","content":"%s"}`, encoded)
	_, err = defaultGHAPI(ctx, "PUT", "/repos/"+repo+"/contents/.github/CODEOWNERS", body)
	if err != nil {
		return result(step, "WARN", err.Error())
	}
	return result(step, "OK", "created .github/CODEOWNERS")
}

func stepBranchProtection(ctx context.Context, repo string, policy hardenPolicy) hardenStepResult {
	step := "branch-protection"
	if hardenDryRun {
		return result(step, "SKIP", "dry-run")
	}

	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return result(step, "FAIL", "invalid repo format: need owner/repo")
	}
	owner, name := parts[0], parts[1]
	branch := policy.DefaultBranch

	requireReviews := "null"
	if policy.RequireReviews {
		requireReviews = fmt.Sprintf(
			`{requiredApprovingReviewCount:%d,dismissesStaleReviews:%t,requiresCodeOwnerReviews:true}`,
			policy.RequiredApprovals, policy.DismissStaleReviews,
		)
	}

	query := fmt.Sprintf(`{"query":"mutation { createBranchProtectionRule(input:{repositoryId:\"%s\",pattern:\"%s\",requiresApprovingReviews:%t,requiredApprovingReviewCount:%d,dismissesStaleReviews:%t,requiresStatusChecks:false,isAdminEnforced:false,allowsForcePushes:false,allowsDeletions:false}) { branchProtectionRule { id } } }"}`,
		getRepoNodeID(ctx, owner, name), branch,
		policy.RequireReviews, policy.RequiredApprovals, policy.DismissStaleReviews,
	)
	_ = requireReviews

	_, err := defaultGHGraphQL(ctx, query)
	if err != nil {
		return result(step, "WARN", err.Error())
	}
	reviewLabel := "no review required"
	if policy.RequireReviews {
		reviewLabel = fmt.Sprintf("%d approval(s) required", policy.RequiredApprovals)
	}
	return result(step, "OK", fmt.Sprintf("protected %s: %s", branch, reviewLabel))
}

func stepTagProtection(ctx context.Context, repo string, policy hardenPolicy) hardenStepResult {
	step := "tag-protection"
	if hardenDryRun {
		return result(step, "SKIP", "dry-run")
	}
	pattern := policy.TagPattern
	if pattern == "" {
		pattern = "v*"
	}
	body := fmt.Sprintf(`{"pattern":"%s"}`, pattern)
	_, err := defaultGHAPI(ctx, "POST", "/repos/"+repo+"/tags/protection", body)
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "already exists") || strings.Contains(errStr, "409") {
			return result(step, "OK", pattern+" already protected")
		}
		return result(step, "WARN", errStr)
	}
	return result(step, "OK", pattern+" protected")
}

func getRepoNodeID(ctx context.Context, owner, name string) string {
	query := fmt.Sprintf(`{"query":"query { repository(owner:\"%s\",name:\"%s\") { id } }"}`, owner, name)
	out, err := defaultGHGraphQL(ctx, query)
	if err != nil {
		return ""
	}
	var resp struct {
		Data struct {
			Repository struct {
				ID string `json:"id"`
			} `json:"repository"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return ""
	}
	return resp.Data.Repository.ID
}

func encodeBase64(data []byte) string {
	const enc = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var b strings.Builder
	for i := 0; i < len(data); i += 3 {
		n := int(data[i]) << 16
		if i+1 < len(data) {
			n |= int(data[i+1]) << 8
		}
		if i+2 < len(data) {
			n |= int(data[i+2])
		}
		b.WriteByte(enc[(n>>18)&0x3F])
		b.WriteByte(enc[(n>>12)&0x3F])
		if i+1 < len(data) {
			b.WriteByte(enc[(n>>6)&0x3F])
		} else {
			b.WriteByte('=')
		}
		if i+2 < len(data) {
			b.WriteByte(enc[n&0x3F])
		} else {
			b.WriteByte('=')
		}
	}
	return b.String()
}
