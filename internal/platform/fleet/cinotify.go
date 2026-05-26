package fleet

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type CIStatusPoller struct {
	client       *http.Client
	githubToken  string
	owner        string
	repos        []string
	pollInterval time.Duration
	onFailure    func(ctx context.Context, event CIFailureEvent) error
	logger       *slog.Logger
}

type CIFailureEvent struct {
	Repo       string    `json:"repo"`
	RunID      int64     `json:"run_id"`
	Branch     string    `json:"branch"`
	CommitSHA  string    `json:"commit_sha"`
	Conclusion string    `json:"conclusion"`
	URL        string    `json:"url"`
	FailedAt   time.Time `json:"failed_at"`
}

type WorkflowRun struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	HeadBranch string `json:"head_branch"`
	HeadSHA    string `json:"head_sha"`
	HTMLURL    string `json:"html_url"`
	CreatedAt  string `json:"created_at"`
}

type WorkflowRunsResponse struct {
	TotalCount   int           `json:"total_count"`
	WorkflowRuns []WorkflowRun `json:"workflow_runs"`
}

type CIPollerOption func(*CIStatusPoller)

func WithPollInterval(d time.Duration) CIPollerOption {
	return func(p *CIStatusPoller) {
		p.pollInterval = d
	}
}

func WithLogger(l *slog.Logger) CIPollerOption {
	return func(p *CIStatusPoller) {
		p.logger = l
	}
}

func NewCIStatusPoller(token, owner string, repos []string, onFailure func(ctx context.Context, event CIFailureEvent) error, opts ...CIPollerOption) *CIStatusPoller {
	p := &CIStatusPoller{
		client:       &http.Client{Timeout: 30 * time.Second},
		githubToken:  token,
		owner:        owner,
		repos:        repos,
		pollInterval: 5 * time.Minute,
		onFailure:    onFailure,
		logger:       slog.Default(),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *CIStatusPoller) Run(ctx context.Context) error {
	p.logger.Info("CI status poller starting", "repos", p.repos, "interval", p.pollInterval)

	seen := make(map[string]struct{})
	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	if err := p.poll(ctx, seen); err != nil {
		p.logger.Warn("initial poll failed", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("CI status poller stopping")
			return ctx.Err()
		case <-ticker.C:
			if err := p.poll(ctx, seen); err != nil {
				p.logger.Error("poll cycle failed", "error", err)
			}
		}
	}
}

func (p *CIStatusPoller) poll(ctx context.Context, seen map[string]struct{}) error {
	for _, repo := range p.repos {
		runs, err := p.fetchRecentRuns(ctx, repo)
		if err != nil {
			p.logger.Error("failed to fetch runs", "repo", repo, "error", err)
			continue
		}

		for _, run := range runs {
			if run.Status != "completed" {
				continue
			}
			if run.Conclusion != "failure" && run.Conclusion != "timed_out" {
				continue
			}

			key := fmt.Sprintf("%s/%d", repo, run.ID)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}

			event := CIFailureEvent{
				Repo:       repo,
				RunID:      run.ID,
				Branch:     run.HeadBranch,
				CommitSHA:  run.HeadSHA,
				Conclusion: run.Conclusion,
				URL:        run.HTMLURL,
				FailedAt:   time.Now(),
			}

			p.logger.Warn("CI failure detected", "repo", repo, "run_id", run.ID, "branch", run.HeadBranch)
			if err := p.onFailure(ctx, event); err != nil {
				p.logger.Error("failure handler error", "error", err)
			}
		}
	}
	return nil
}

func (p *CIStatusPoller) fetchRecentRuns(ctx context.Context, repo string) ([]WorkflowRun, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/runs?per_page=10&status=completed", p.owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.githubToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d for %s", resp.StatusCode, repo)
	}

	var result WorkflowRunsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result.WorkflowRuns, nil
}
