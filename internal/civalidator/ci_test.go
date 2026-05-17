package civalidator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validWorkflowYAML = `name: CI Pipeline
on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Lint
        run: golangci-lint run

  test:
    runs-on: ubuntu-latest
    needs: [lint]
    steps:
      - uses: actions/checkout@v4
      - name: Test
        run: go test ./... -race -coverprofile=coverage.out

  build:
    runs-on: ubuntu-latest
    needs: [test]
    steps:
      - uses: actions/checkout@v4
      - name: Build Docker
        uses: docker/build-push-action@v5
        with:
          context: .
          file: ./Dockerfile
          push: true
          tags: ghcr.io/org/app:${{ github.sha }}

  deploy:
    runs-on: ubuntu-latest
    needs: [test, build]
    if: github.ref == 'refs/heads/main'
    steps:
      - name: Deploy
        run: kubectl apply -f k8s/
`

func TestParseWorkflow_Valid(t *testing.T) {
	wf, err := ParseGitHubWorkflow([]byte(validWorkflowYAML))
	require.NoError(t, err)
	assert.Equal(t, "CI Pipeline", wf.Name)
	assert.Len(t, wf.Jobs, 4)
	assert.Contains(t, wf.Jobs, "lint")
	assert.Contains(t, wf.Jobs, "test")
	assert.Contains(t, wf.Jobs, "build")
}

func TestParseWorkflow_Invalid(t *testing.T) {
	_, err := ParseGitHubWorkflow([]byte(`not: valid: yaml: [[[`))
	assert.Error(t, err)

	_, err = ParseGitHubWorkflow([]byte(`name: Empty`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no jobs")
}

func TestWorkflowJobs_AllPresent(t *testing.T) {
	wf, err := ParseGitHubWorkflow([]byte(validWorkflowYAML))
	require.NoError(t, err)
	err = ValidateWorkflowJobs(*wf)
	assert.NoError(t, err)
}

func TestWorkflowJobs_MissingTest(t *testing.T) {
	yaml := `name: No Test
on: push
jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - run: golangci-lint run
  build:
    runs-on: ubuntu-latest
    steps:
      - run: docker build .
`
	wf, err := ParseGitHubWorkflow([]byte(yaml))
	require.NoError(t, err)
	err = ValidateWorkflowJobs(*wf)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "test")
}

func TestWorkflowJobs_MissingBuild(t *testing.T) {
	yaml := `name: No Build
on: push
jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - run: golangci-lint run
  test:
    runs-on: ubuntu-latest
    steps:
      - run: go test ./...
`
	wf, err := ParseGitHubWorkflow([]byte(yaml))
	require.NoError(t, err)
	err = ValidateWorkflowJobs(*wf)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "build")
}

func TestDockerBuild_Valid(t *testing.T) {
	wf, err := ParseGitHubWorkflow([]byte(validWorkflowYAML))
	require.NoError(t, err)
	err = ValidateDockerBuildStep(wf.Jobs["build"])
	assert.NoError(t, err)
}

func TestDockerBuild_MissingTag(t *testing.T) {
	yaml := `name: No Tag
on: push
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: docker/build-push-action@v5
        with:
          context: .
          push: true
`
	wf, err := ParseGitHubWorkflow([]byte(yaml))
	require.NoError(t, err)
	err = ValidateDockerBuildStep(wf.Jobs["build"])
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tags")
}

func TestDockerBuild_NoPush(t *testing.T) {
	yaml := `name: No Push
on: push
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: docker/build-push-action@v5
        with:
          context: .
          tags: ghcr.io/org/app:latest
`
	wf, err := ParseGitHubWorkflow([]byte(yaml))
	require.NoError(t, err)
	err = ValidateDockerBuildStep(wf.Jobs["build"])
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "push")
}

func TestTestStep_WithRace(t *testing.T) {
	wf, err := ParseGitHubWorkflow([]byte(validWorkflowYAML))
	require.NoError(t, err)
	err = ValidateTestStep(wf.Jobs["test"])
	assert.NoError(t, err)
}

func TestTestStep_WithCoverage(t *testing.T) {
	yaml := `name: Coverage
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - run: go test ./... -coverprofile=coverage.out
`
	wf, err := ParseGitHubWorkflow([]byte(yaml))
	require.NoError(t, err)
	err = ValidateTestStep(wf.Jobs["test"])
	assert.NoError(t, err)
}

func TestTestStep_Bare(t *testing.T) {
	yaml := `name: Bare Test
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - run: go test ./...
`
	wf, err := ParseGitHubWorkflow([]byte(yaml))
	require.NoError(t, err)
	err = ValidateTestStep(wf.Jobs["test"])
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "-race")
}

func TestReleaseGate_Valid(t *testing.T) {
	wf, err := ParseGitHubWorkflow([]byte(validWorkflowYAML))
	require.NoError(t, err)
	err = ValidateReleaseGate(*wf)
	assert.NoError(t, err)
}

func TestReleaseGate_NoGate(t *testing.T) {
	yaml := `name: No Gate
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - run: go test ./...
  deploy:
    runs-on: ubuntu-latest
    steps:
      - run: kubectl apply -f k8s/
`
	wf, err := ParseGitHubWorkflow([]byte(yaml))
	require.NoError(t, err)
	err = ValidateReleaseGate(*wf)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "needs")
}
