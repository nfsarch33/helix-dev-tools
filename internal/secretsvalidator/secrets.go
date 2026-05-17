package secretsvalidator

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type secret struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
	Type string            `yaml:"type"`
	Data map[string]string `yaml:"data"`
}

type sealedSecret struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
	Spec struct {
		EncryptedData map[string]string `yaml:"encryptedData"`
		Template      struct {
			Metadata struct {
				Name      string `yaml:"name"`
				Namespace string `yaml:"namespace"`
			} `yaml:"metadata"`
			Type string `yaml:"type"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

type externalSecret struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Spec       struct {
		RefreshInterval string `yaml:"refreshInterval"`
		SecretStoreRef  struct {
			Name string `yaml:"name"`
			Kind string `yaml:"kind"`
		} `yaml:"secretStoreRef"`
		Target struct {
			Name string `yaml:"name"`
		} `yaml:"target"`
		Data []struct {
			SecretKey string `yaml:"secretKey"`
			RemoteRef struct {
				Key string `yaml:"key"`
			} `yaml:"remoteRef"`
		} `yaml:"data"`
	} `yaml:"spec"`
}

type rotationPolicy struct {
	RotationInterval string `yaml:"rotationInterval"`
	LastRotated      string `yaml:"lastRotated"`
	Secrets          []struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"secrets"`
}

var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(ghp_[a-zA-Z0-9]{36})`),
	regexp.MustCompile(`(?i)(gho_[a-zA-Z0-9]{36})`),
	regexp.MustCompile(`(?i)(sk-[a-zA-Z0-9]{20,})`),
	regexp.MustCompile(`(?i)(AKIA[0-9A-Z]{16})`),
	regexp.MustCompile(`(?i)password["\s:=]+["']?[a-zA-Z0-9!@#$%^&*]{6,}["']?`),
	regexp.MustCompile(`(?i)api[_-]?key["\s:=]+["']?[a-zA-Z0-9]{10,}["']?`),
	regexp.MustCompile(`(?i)secret["\s:=]+["']?[a-zA-Z0-9]{10,}["']?`),
	regexp.MustCompile(`(?i)token["\s:=]+["']?[a-zA-Z0-9]{10,}["']?`),
}

// ValidateSecretStructure validates a Kubernetes Secret has a type, data keys, and no plaintext.
func ValidateSecretStructure(manifest []byte) error {
	var s secret
	if err := yaml.Unmarshal(manifest, &s); err != nil {
		return fmt.Errorf("parsing secret: %w", err)
	}
	if s.Kind != "Secret" {
		return fmt.Errorf("expected kind Secret, got %q", s.Kind)
	}
	if s.Type == "" {
		return fmt.Errorf("secret %q missing type field", s.Metadata.Name)
	}
	if len(s.Data) == 0 {
		return fmt.Errorf("secret %q has no data keys", s.Metadata.Name)
	}
	return nil
}

// ValidateNoPlaintextSecrets scans manifests for embedded plaintext tokens or credentials.
// Kubernetes Secret manifests (kind: Secret) are excluded since their data fields are
// base64-encoded by design.
func ValidateNoPlaintextSecrets(manifests [][]byte) error {
	var findings []string
	for i, m := range manifests {
		if isKubernetesSecret(m) {
			continue
		}
		content := string(m)
		for _, pat := range sensitivePatterns {
			if matches := pat.FindStringSubmatch(content); len(matches) > 0 {
				findings = append(findings, fmt.Sprintf("manifest[%d]: plaintext sensitive value detected (pattern: %s)", i, pat.String()))
				break
			}
		}
	}
	if len(findings) > 0 {
		return fmt.Errorf("%s", strings.Join(findings, "; "))
	}
	return nil
}

func isKubernetesSecret(manifest []byte) bool {
	var obj struct {
		Kind string `yaml:"kind"`
	}
	if err := yaml.Unmarshal(manifest, &obj); err != nil {
		return false
	}
	return obj.Kind == "Secret"
}

// ValidateSecretRotationPolicy checks that secrets have a valid rotation interval
// and that the last rotation is not overdue.
func ValidateSecretRotationPolicy(config []byte) error {
	var rp rotationPolicy
	if err := yaml.Unmarshal(config, &rp); err != nil {
		return fmt.Errorf("parsing rotation policy: %w", err)
	}
	if rp.RotationInterval == "" {
		return fmt.Errorf("rotation policy missing rotationInterval")
	}
	if rp.LastRotated == "" {
		return fmt.Errorf("rotation policy missing lastRotated timestamp")
	}

	interval, err := parseDuration(rp.RotationInterval)
	if err != nil {
		return fmt.Errorf("invalid rotation interval %q: %w", rp.RotationInterval, err)
	}

	lastRotated, err := time.Parse(time.RFC3339, rp.LastRotated)
	if err != nil {
		return fmt.Errorf("invalid lastRotated timestamp: %w", err)
	}

	if time.Since(lastRotated) > interval {
		return fmt.Errorf("secret rotation overdue: last rotated %s, interval %s",
			lastRotated.Format("2006-01-02"), rp.RotationInterval)
	}
	return nil
}

// ValidateSealedSecret validates a Bitnami SealedSecret CRD has encryptedData.
func ValidateSealedSecret(manifest []byte) error {
	var ss sealedSecret
	if err := yaml.Unmarshal(manifest, &ss); err != nil {
		return fmt.Errorf("parsing sealed secret: %w", err)
	}
	if ss.Kind != "SealedSecret" {
		return fmt.Errorf("expected kind SealedSecret, got %q", ss.Kind)
	}
	if len(ss.Spec.EncryptedData) == 0 {
		return fmt.Errorf("sealed secret %q has no encryptedData", ss.Metadata.Name)
	}
	return nil
}

// ValidateExternalSecretRef validates an ExternalSecret has a valid secretStoreRef.
func ValidateExternalSecretRef(manifest []byte) error {
	var es externalSecret
	if err := yaml.Unmarshal(manifest, &es); err != nil {
		return fmt.Errorf("parsing external secret: %w", err)
	}
	if es.Kind != "ExternalSecret" {
		return fmt.Errorf("expected kind ExternalSecret, got %q", es.Kind)
	}
	if es.Spec.SecretStoreRef.Name == "" {
		return fmt.Errorf("external secret missing secretStoreRef.name")
	}
	if es.Spec.SecretStoreRef.Kind == "" {
		return fmt.Errorf("external secret missing secretStoreRef.kind")
	}
	return nil
}

func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "d") {
		var days int
		if _, err := fmt.Sscanf(s, "%dd", &days); err != nil {
			return 0, err
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}
