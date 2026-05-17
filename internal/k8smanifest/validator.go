package k8smanifest

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

var memoryRe = regexp.MustCompile(`^(\d+)(Gi|Mi|Ki)?$`)

// ValidateOmniParser checks an OmniParser Deployment manifest against requirements.
func ValidateOmniParser(data []byte, reqs OmniParserRequirements) (*ValidationResult, error) {
	docs, err := splitYAMLDocs(data)
	if err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}

	var dep *ManifestDoc
	for i := range docs {
		if docs[i].Kind == "Deployment" {
			dep = &docs[i]
			break
		}
	}
	if dep == nil {
		return nil, fmt.Errorf("no Deployment found in manifest")
	}

	result := &ValidationResult{Name: "OmniParser"}

	if reqs.Namespace != "" && dep.Metadata.Namespace != reqs.Namespace {
		result.Failures = append(result.Failures,
			fmt.Sprintf("namespace mismatch: want %q, got %q", reqs.Namespace, dep.Metadata.Namespace))
	} else {
		result.Passed = append(result.Passed, "namespace OK")
	}

	containers := extractContainers(dep)
	if len(containers) == 0 {
		result.Failures = append(result.Failures, "no containers found in pod spec")
		return result, nil
	}
	c := containers[0]

	gpuOK := checkGPU(c, reqs.MinGPU)
	if gpuOK {
		result.Passed = append(result.Passed, "GPU resource requests present")
	} else {
		result.Failures = append(result.Failures,
			fmt.Sprintf("GPU resources missing or below minimum (%d)", reqs.MinGPU))
	}

	for _, probe := range reqs.RequiredProbes {
		if _, ok := c[probe]; ok {
			result.Passed = append(result.Passed, probe+" configured")
		} else {
			result.Failures = append(result.Failures, probe+" not configured")
		}
	}

	volumeOK := checkModelVolume(c, dep, reqs.ModelVolumePath)
	if volumeOK {
		result.Passed = append(result.Passed, "model volume mount at "+reqs.ModelVolumePath)
	} else {
		result.Failures = append(result.Failures,
			fmt.Sprintf("model volume mount at %s not found", reqs.ModelVolumePath))
	}

	memOK := checkMemoryLimit(c, reqs.MinMemoryLimitMi)
	if memOK {
		result.Passed = append(result.Passed, "memory limit meets minimum")
	} else {
		result.Failures = append(result.Failures,
			fmt.Sprintf("memory limit below minimum %dMi", reqs.MinMemoryLimitMi))
	}

	return result, nil
}

func splitYAMLDocs(data []byte) ([]ManifestDoc, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var docs []ManifestDoc
	for {
		var doc ManifestDoc
		err := decoder.Decode(&doc)
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return docs, err
		}
		if doc.Kind != "" {
			docs = append(docs, doc)
		}
	}
	return docs, nil
}

func extractContainers(dep *ManifestDoc) []map[string]any {
	spec, ok := dep.Spec["template"]
	if !ok {
		return nil
	}
	tmpl, ok := spec.(map[string]any)
	if !ok {
		return nil
	}
	podSpec, ok := tmpl["spec"]
	if !ok {
		return nil
	}
	ps, ok := podSpec.(map[string]any)
	if !ok {
		return nil
	}
	cs, ok := ps["containers"]
	if !ok {
		return nil
	}
	cList, ok := cs.([]any)
	if !ok {
		return nil
	}
	var out []map[string]any
	for _, c := range cList {
		if cm, ok := c.(map[string]any); ok {
			out = append(out, cm)
		}
	}
	return out
}

func checkGPU(container map[string]any, minGPU int) bool {
	for _, section := range []string{"requests", "limits"} {
		resources, ok := container["resources"]
		if !ok {
			continue
		}
		rm, ok := resources.(map[string]any)
		if !ok {
			continue
		}
		sec, ok := rm[section]
		if !ok {
			continue
		}
		sm, ok := sec.(map[string]any)
		if !ok {
			continue
		}
		gpuVal, ok := sm["nvidia.com/gpu"]
		if !ok {
			continue
		}
		gpuStr := fmt.Sprintf("%v", gpuVal)
		gpuCount, err := strconv.Atoi(gpuStr)
		if err != nil {
			continue
		}
		if gpuCount >= minGPU {
			return true
		}
	}
	return false
}

func checkModelVolume(container map[string]any, dep *ManifestDoc, mountPath string) bool {
	mounts, ok := container["volumeMounts"]
	if !ok {
		return false
	}
	mountList, ok := mounts.([]any)
	if !ok {
		return false
	}
	for _, m := range mountList {
		mm, ok := m.(map[string]any)
		if !ok {
			continue
		}
		if mp, ok := mm["mountPath"]; ok && fmt.Sprintf("%v", mp) == mountPath {
			return true
		}
	}
	return false
}

func checkMemoryLimit(container map[string]any, minMi int) bool {
	resources, ok := container["resources"]
	if !ok {
		return false
	}
	rm, ok := resources.(map[string]any)
	if !ok {
		return false
	}
	limits, ok := rm["limits"]
	if !ok {
		return false
	}
	lm, ok := limits.(map[string]any)
	if !ok {
		return false
	}
	memVal, ok := lm["memory"]
	if !ok {
		return false
	}
	return parseMemoryMi(fmt.Sprintf("%v", memVal)) >= minMi
}

func parseMemoryMi(s string) int {
	s = strings.TrimSpace(s)
	matches := memoryRe.FindStringSubmatch(s)
	if matches == nil {
		return 0
	}
	val, _ := strconv.Atoi(matches[1])
	switch matches[2] {
	case "Gi":
		return val * 1024
	case "Mi":
		return val
	case "Ki":
		return val / 1024
	default:
		return val / (1024 * 1024)
	}
}
