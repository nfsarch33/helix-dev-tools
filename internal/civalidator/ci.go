package civalidator

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Workflow represents a GitHub Actions workflow file.
type Workflow struct {
	Name string         `yaml:"name"`
	On   interface{}    `yaml:"on"`
	Jobs map[string]Job `yaml:"jobs"`
}

// Job represents a single job within a GitHub Actions workflow.
type Job struct {
	RunsOn string   `yaml:"runs-on"`
	Needs  yamlList `yaml:"needs"`
	If     string   `yaml:"if"`
	Steps  []Step   `yaml:"steps"`
}

// Step represents a single step within a workflow job.
type Step struct {
	Name string            `yaml:"name"`
	Uses string            `yaml:"uses"`
	Run  string            `yaml:"run"`
	With map[string]string `yaml:"with"`
}

// yamlList handles both string and []string YAML values for the needs field.
type yamlList []string

func (yl *yamlList) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		*yl = []string{value.Value}
		return nil
	}
	var list []string
	if err := value.Decode(&list); err != nil {
		return err
	}
	*yl = list
	return nil
}

// ParseGitHubWorkflow parses a GitHub Actions workflow YAML file.
func ParseGitHubWorkflow(yamlBytes []byte) (*Workflow, error) {
	var wf Workflow
	if err := yaml.Unmarshal(yamlBytes, &wf); err != nil {
		return nil, fmt.Errorf("parsing workflow: %w", err)
	}
	if len(wf.Jobs) == 0 {
		return nil, fmt.Errorf("workflow %q has no jobs defined", wf.Name)
	}
	return &wf, nil
}

// ValidateWorkflowJobs checks that required jobs (lint, test, build) are present.
func ValidateWorkflowJobs(workflow Workflow) error {
	required := []string{"lint", "test", "build"}
	var missing []string
	for _, name := range required {
		if _, ok := workflow.Jobs[name]; !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("workflow missing required jobs: [%s]", strings.Join(missing, ", "))
	}
	return nil
}

// ValidateDockerBuildStep checks that a build job uses docker/build-push-action
// with tags and push enabled.
func ValidateDockerBuildStep(job Job) error {
	for _, step := range job.Steps {
		if !strings.Contains(step.Uses, "docker/build-push-action") {
			continue
		}
		if step.With["tags"] == "" {
			return fmt.Errorf("docker build step missing tags")
		}
		if step.With["push"] != "true" {
			return fmt.Errorf("docker build step has push disabled or missing")
		}
		return nil
	}

	for _, step := range job.Steps {
		if strings.Contains(step.Run, "docker build") {
			if !strings.Contains(step.Run, "-t ") && !strings.Contains(step.Run, "--tag") {
				return fmt.Errorf("docker build command missing tags")
			}
			return nil
		}
	}

	return fmt.Errorf("no docker build step found in job")
}

// ValidateTestStep checks that a test job includes -race and/or -coverprofile flags.
func ValidateTestStep(job Job) error {
	for _, step := range job.Steps {
		if step.Run == "" {
			continue
		}
		if strings.Contains(step.Run, "test") {
			if strings.Contains(step.Run, "-race") || strings.Contains(step.Run, "-coverprofile") {
				return nil
			}
			return fmt.Errorf("test step missing -race or -coverprofile flag: %q", step.Run)
		}
	}
	return fmt.Errorf("no test command found in job steps")
}

// ValidateReleaseGate checks that deploy/release jobs depend on the test job.
func ValidateReleaseGate(workflow Workflow) error {
	deployJobNames := []string{"deploy", "release", "publish"}
	for _, name := range deployJobNames {
		job, ok := workflow.Jobs[name]
		if !ok {
			continue
		}
		hasTestDep := false
		for _, need := range job.Needs {
			if need == "test" {
				hasTestDep = true
				break
			}
		}
		if !hasTestDep {
			return fmt.Errorf("job %q missing needs dependency on test job", name)
		}
	}
	return nil
}
