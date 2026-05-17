package cli

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/helix-dev-tools/internal/workspace"
)

var botPRsDryRun bool
var botPRsRepoAlias string
var botPRsAll bool
var botPRsConfig string

var knownBotAuthors = []string{
	"dependabot[bot]",
	"github-actions[bot]",
	"app/dependabot",
}

type ghPR struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	Author    string `json:"author"`
	Mergeable string `json:"mergeable"`
	Repo      string `json:"-"`
}

var botPRsCmd = &cobra.Command{
	Use:   "bot-prs",
	Short: "Scan and merge bot PRs (Dependabot, GitHub Actions) across owned repos",
}

var botPRsScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan all owned repos for open bot PRs",
	RunE:  runBotPRsScan,
}

var botPRsMergeCmd = &cobra.Command{
	Use:   "merge",
	Short: "Merge safe bot PRs (patch/minor bumps without conflicts)",
	RunE:  runBotPRsMerge,
}

func init() {
	botPRsCmd.AddCommand(botPRsScanCmd)
	botPRsCmd.AddCommand(botPRsMergeCmd)

	botPRsCmd.PersistentFlags().BoolVar(&botPRsDryRun, "dry-run", false, "Preview actions without merging")
	botPRsCmd.PersistentFlags().StringVar(&botPRsConfig, "config", "", "runx config path")

	botPRsMergeCmd.Flags().StringVar(&botPRsRepoAlias, "repo", "", "Merge bot PRs for a specific repo alias")
	botPRsMergeCmd.Flags().BoolVar(&botPRsAll, "all", false, "Merge bot PRs across all repos")
}

func runBotPRsScan(_ *cobra.Command, _ []string) error {
	repos, err := discoverRepos()
	if err != nil {
		return err
	}

	total := 0
	for _, repo := range repos {
		prs, err := listBotPRs(repo)
		if err != nil {
			fmt.Printf("WARN: failed to list PRs for %s: %v\n", repo, err)
			continue
		}
		if len(prs) == 0 {
			continue
		}
		fmt.Printf("\n--- %s ---\n", repo)
		for _, pr := range prs {
			verdict := classifyPR(pr)
			fmt.Printf("  #%-4d [%s] %s\n", pr.Number, verdict, pr.Title)
		}
		total += len(prs)
	}

	fmt.Printf("\nTotal bot PRs found: %d\n", total)
	return nil
}

func runBotPRsMerge(_ *cobra.Command, _ []string) error {
	if !botPRsAll && botPRsRepoAlias == "" {
		return fmt.Errorf("specify --repo <alias> or --all")
	}

	var repos []string
	if botPRsAll {
		var err error
		repos, err = discoverRepos()
		if err != nil {
			return err
		}
	} else {
		resolved, err := resolveRepoAlias(botPRsRepoAlias)
		if err != nil {
			return err
		}
		repos = []string{resolved}
	}

	merged := 0
	skipped := 0
	for _, repo := range repos {
		prs, err := listBotPRs(repo)
		if err != nil {
			fmt.Printf("WARN: failed to list PRs for %s: %v\n", repo, err)
			continue
		}
		for _, pr := range prs {
			verdict := classifyPR(pr)
			if verdict == "SKIP" {
				fmt.Printf("SKIP  %s #%d: %s (major version bump)\n", repo, pr.Number, pr.Title)
				skipped++
				continue
			}
			if pr.Mergeable == "CONFLICTING" {
				fmt.Printf("SKIP  %s #%d: %s (merge conflict)\n", repo, pr.Number, pr.Title)
				skipped++
				continue
			}
			if botPRsDryRun {
				fmt.Printf("WOULD MERGE  %s #%d: %s\n", repo, pr.Number, pr.Title)
				merged++
				continue
			}
			if err := mergePR(repo, pr.Number); err != nil {
				fmt.Printf("ERROR %s #%d: %v\n", repo, pr.Number, err)
				skipped++
				continue
			}
			fmt.Printf("MERGED %s #%d: %s\n", repo, pr.Number, pr.Title)
			merged++
		}
	}

	action := "Merged"
	if botPRsDryRun {
		action = "Would merge"
	}
	fmt.Printf("\n%s: %d | Skipped: %d\n", action, merged, skipped)
	return nil
}

func discoverRepos() ([]string, error) {
	loaded, err := workspace.LoadRunxRepos(botPRsConfig, "", nil)
	if err != nil {
		return nil, fmt.Errorf("bot-prs: loading repos from runx config: %w", err)
	}
	repos := make([]string, 0, len(loaded))
	for _, r := range loaded {
		slug := resolveSlug(r.Alias)
		if slug != "" {
			repos = append(repos, slug)
		}
	}
	if len(repos) == 0 {
		return discoverReposFromGH()
	}
	return repos, nil
}

func discoverReposFromGH() ([]string, error) {
	out, err := exec.Command("gh", "repo", "list", "nfsarch33",
		"--limit", "100", "--json", "nameWithOwner",
		"--jq", ".[].nameWithOwner").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gh repo list: %w: %s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	repos := make([]string, 0, len(lines))
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			repos = append(repos, l)
		}
	}
	return repos, nil
}

func resolveRepoAlias(alias string) (string, error) {
	loaded, err := workspace.LoadRunxRepos(botPRsConfig, "", []string{alias})
	if err != nil || len(loaded) == 0 {
		return "nfsarch33/" + alias, nil
	}
	slug := resolveSlug(loaded[0].Alias)
	if slug == "" {
		return "nfsarch33/" + alias, nil
	}
	return slug, nil
}

func resolveSlug(alias string) string {
	return "nfsarch33/" + alias
}

type ghPRRaw struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Author    ghAuthor  `json:"author"`
	Mergeable string    `json:"mergeable"`
	Labels    []ghLabel `json:"labels"`
}

type ghAuthor struct {
	Login string `json:"login"`
}

type ghLabel struct {
	Name string `json:"name"`
}

func listBotPRs(repo string) ([]ghPR, error) {
	out, err := exec.Command("gh", "pr", "list",
		"--repo", repo,
		"--state", "open",
		"--json", "number,title,author,mergeable,labels").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gh pr list: %w", err)
	}

	var raw []ghPRRaw
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parse gh output: %w", err)
	}

	var prs []ghPR
	for _, r := range raw {
		if !isBotAuthor(r.Author.Login) {
			continue
		}
		prs = append(prs, ghPR{
			Number:    r.Number,
			Title:     r.Title,
			Author:    r.Author.Login,
			Mergeable: r.Mergeable,
			Repo:      repo,
		})
	}
	return prs, nil
}

func isBotAuthor(login string) bool {
	for _, bot := range knownBotAuthors {
		if login == bot {
			return true
		}
	}
	return false
}

// classifyPR returns "MERGE" for safe PRs (patch/minor bumps) or "SKIP" for major bumps.
func classifyPR(pr ghPR) string {
	title := strings.ToLower(pr.Title)
	if isMajorBump(title) {
		return "SKIP"
	}
	return "MERGE"
}

func isMajorBump(title string) bool {
	parts := strings.Fields(title)
	if len(parts) < 2 {
		return false
	}
	last := parts[len(parts)-1]
	fromTo := strings.Split(last, " to ")
	if len(fromTo) < 2 {
		for i, p := range parts {
			if p == "from" && i+1 < len(parts) && i+3 < len(parts) && parts[i+2] == "to" {
				return isMajorVersionChange(parts[i+1], parts[i+3])
			}
		}
		return false
	}
	return false
}

func isMajorVersionChange(from, to string) bool {
	fromMajor := extractMajor(from)
	toMajor := extractMajor(to)
	if fromMajor == "" || toMajor == "" {
		return false
	}
	return fromMajor != toMajor
}

func extractMajor(ver string) string {
	ver = strings.TrimPrefix(ver, "v")
	parts := strings.SplitN(ver, ".", 2)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func mergePR(repo string, number int) error {
	out, err := exec.Command("gh", "pr", "merge",
		fmt.Sprintf("%d", number),
		"--repo", repo,
		"--squash",
		"--auto").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(out))
	}
	return nil
}
