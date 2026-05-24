package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/nfsarch33/helix-dev-tools/internal/dashboard"
)

func main() {
	addr := flag.String("addr", ":9090", "listen address")
	gitlabURL := flag.String("gitlab-url", "http://localhost:30080", "GitLab base URL")
	gitlabToken := flag.String("gitlab-token", "", "GitLab private token")
	gitlabProjects := flag.String("gitlab-projects", "", "comma-separated GitLab project IDs")
	argoURL := flag.String("argocd-url", "http://localhost:30880", "ArgoCD base URL")
	argoToken := flag.String("argocd-token", "", "ArgoCD bearer token")
	engramURL := flag.String("engram-url", "http://localhost:8281", "Engram base URL")
	manifestPath := flag.String("manifest", "", "roadmap YAML manifest path")
	flag.Parse()

	var fetchers []dashboard.Fetcher

	pids := parseProjectIDs(*gitlabProjects)
	if len(pids) > 0 {
		fetchers = append(fetchers, &dashboard.GitLabFetcher{
			BaseURL:    *gitlabURL,
			ProjectIDs: pids,
			Token:      *gitlabToken,
			Client:     &http.Client{},
		})
	}

	fetchers = append(fetchers, &dashboard.ArgoCDFetcher{
		BaseURL: *argoURL,
		Token:   *argoToken,
		Client:  &http.Client{},
	})

	fetchers = append(fetchers, &dashboard.SprintBoardFetcher{})

	fetchers = append(fetchers, &dashboard.EngramFetcher{
		BaseURL: *engramURL,
		Client:  &http.Client{},
	})

	fetchers = append(fetchers, &dashboard.AgentraceFetcher{})

	srv, err := dashboard.New(fetchers, *manifestPath, *addr)
	if err != nil {
		log.Fatalf("dashboard init: %v", err)
	}

	if err := srv.ListenAndServe(); err != nil {
		log.Printf("dashboard exited: %v", err)
		os.Exit(1)
	}
}

func parseProjectIDs(s string) []int {
	if s == "" {
		return nil
	}
	var ids []int
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if id, err := strconv.Atoi(part); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}
