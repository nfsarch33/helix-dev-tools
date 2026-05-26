package cli

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/nfsarch33/helix-dev-tools/internal/dashboard"
	"github.com/spf13/cobra"
)

var (
	dashboardPort      int
	dashboardManifest  string
	dashboardEngramURL string
)

var dashboardServeCmd = &cobra.Command{
	Use:   "dashboard-serve",
	Short: "Start the helix dashboard HTTP server",
	Long: `Serves the helix dashboard on the specified port.
Wraps the same HTTP server as the standalone helix-dashboard binary.`,
	RunE: runDashboardServe,
}

func init() {
	dashboardServeCmd.Flags().IntVar(&dashboardPort, "port", 9095, "HTTP listen port")
	dashboardServeCmd.Flags().StringVar(&dashboardManifest, "manifest", "", "roadmap YAML manifest path")
	dashboardServeCmd.Flags().StringVar(&dashboardEngramURL, "engram-url", "http://localhost:8281", "Engram base URL")
}

// dashboardEnvFn allows tests to override env var lookups.
var dashboardEnvFn = os.Getenv

func runDashboardServe(cmd *cobra.Command, _ []string) error {
	out := cmd.OutOrStdout()

	fetchers := buildDashboardFetchers()
	addr := ":" + strconv.Itoa(dashboardPort)

	srv, err := dashboard.New(fetchers, dashboardManifest, addr)
	if err != nil {
		return fmt.Errorf("dashboard init: %w", err)
	}

	fmt.Fprintf(out, "dashboard listening on %s\n", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("dashboard serve: %w", err)
	}
	return nil
}

func buildDashboardFetchers() []dashboard.Fetcher {
	var fetchers []dashboard.Fetcher

	fetchers = append(fetchers, &dashboard.ArgoCDFetcher{
		BaseURL: envFallback("ARGOCD_URL", "http://localhost:30880"),
		Token:   dashboardEnvFn("ARGOCD_TOKEN"),
		Client:  &http.Client{},
	})

	fetchers = append(fetchers, &dashboard.SprintBoardFetcher{})

	fetchers = append(fetchers, &dashboard.EngramFetcher{
		BaseURL: dashboardEngramURL,
		Client:  &http.Client{},
	})

	fetchers = append(fetchers, &dashboard.AgentraceFetcher{})

	gitlabProjects := dashboardEnvFn("GITLAB_PROJECTS")
	if gitlabProjects != "" {
		pids := parseDashboardProjectIDs(gitlabProjects)
		if len(pids) > 0 {
			fetchers = append(fetchers, &dashboard.GitLabFetcher{
				BaseURL:    envFallback("GITLAB_URL", "http://localhost:30080"),
				ProjectIDs: pids,
				Token:      dashboardEnvFn("GITLAB_TOKEN"),
				Client:     &http.Client{},
			})
		}
	}

	return fetchers
}

func envFallback(key, fallback string) string {
	if v := dashboardEnvFn(key); v != "" {
		return v
	}
	return fallback
}

func parseDashboardProjectIDs(s string) []int {
	var ids []int
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if id, err := strconv.Atoi(part); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}
