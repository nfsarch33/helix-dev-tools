package secretsvalidator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validOpaqueSecretYAML = `apiVersion: v1
kind: Secret
metadata:
  name: db-credentials
  namespace: production
type: Opaque
data:
  username: YWRtaW4=
  password: cDRzc3cwcmQ=
`

const validTLSSecretYAML = `apiVersion: v1
kind: Secret
metadata:
  name: tls-cert
  namespace: production
type: kubernetes.io/tls
data:
  tls.crt: LS0tLS1CRUdJTi...
  tls.key: LS0tLS1CRUdJTi...
`

func TestValidateSecretStructure_Opaque(t *testing.T) {
	err := ValidateSecretStructure([]byte(validOpaqueSecretYAML))
	assert.NoError(t, err)
}

func TestValidateSecretStructure_TLS(t *testing.T) {
	err := ValidateSecretStructure([]byte(validTLSSecretYAML))
	assert.NoError(t, err)
}

func TestValidateSecretStructure_Invalid(t *testing.T) {
	err := ValidateSecretStructure([]byte(`apiVersion: v1
kind: Secret
metadata:
  name: bad-secret
`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "type")

	err = ValidateSecretStructure([]byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: not-a-secret
`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Secret")
}

func TestNoPlaintext_Clean(t *testing.T) {
	manifests := [][]byte{
		[]byte(validOpaqueSecretYAML),
		[]byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  template:
    spec:
      containers:
        - name: web
          image: nginx:1.25
`),
	}
	err := ValidateNoPlaintextSecrets(manifests)
	assert.NoError(t, err)
}

func TestNoPlaintext_TokenLeak(t *testing.T) {
	manifests := [][]byte{
		[]byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  template:
    spec:
      containers:
        - name: web
          image: nginx
          env:
            - name: API_KEY
              value: "ghp_abc123def456ghi789jkl012mno345pqr678"
`),
	}
	err := ValidateNoPlaintextSecrets(manifests)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "plaintext")
}

func TestNoPlaintext_Base64Encoded(t *testing.T) {
	manifests := [][]byte{
		[]byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  template:
    spec:
      containers:
        - name: web
          image: nginx
          env:
            - name: AWS_ACCESS_KEY
              value: "AKIAIOSFODNN7EXAMPLE"
`),
	}
	err := ValidateNoPlaintextSecrets(manifests)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plaintext")
}

const validRotationPolicyYAML = `rotationInterval: 90d
lastRotated: "2026-04-15T00:00:00Z"
secrets:
  - name: db-credentials
    namespace: production
  - name: api-key
    namespace: production
`

func TestRotationPolicy_Valid(t *testing.T) {
	err := ValidateSecretRotationPolicy([]byte(validRotationPolicyYAML))
	assert.NoError(t, err)
}

func TestRotationPolicy_Expired(t *testing.T) {
	expired := `rotationInterval: 30d
lastRotated: "2025-01-01T00:00:00Z"
secrets:
  - name: old-key
    namespace: production
`
	err := ValidateSecretRotationPolicy([]byte(expired))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "overdue")
}

const validSealedSecretYAML = `apiVersion: bitnami.com/v1alpha1
kind: SealedSecret
metadata:
  name: my-secret
  namespace: production
spec:
  encryptedData:
    username: AgBY2...
    password: AgCX3...
  template:
    metadata:
      name: my-secret
      namespace: production
    type: Opaque
`

func TestSealedSecret_Valid(t *testing.T) {
	err := ValidateSealedSecret([]byte(validSealedSecretYAML))
	assert.NoError(t, err)
}

func TestSealedSecret_MissingSealedData(t *testing.T) {
	yaml := `apiVersion: bitnami.com/v1alpha1
kind: SealedSecret
metadata:
  name: empty-sealed
  namespace: production
spec:
  template:
    metadata:
      name: empty-sealed
`
	err := ValidateSealedSecret([]byte(yaml))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "encryptedData")
}

const validExternalSecretYAML = `apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: db-creds
  namespace: production
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: aws-secrets-manager
    kind: SecretStore
  target:
    name: db-credentials
  data:
    - secretKey: password
      remoteRef:
        key: prod/db/password
`

func TestExternalSecretRef_Valid(t *testing.T) {
	err := ValidateExternalSecretRef([]byte(validExternalSecretYAML))
	assert.NoError(t, err)
}

func TestExternalSecretRef_MissingProvider(t *testing.T) {
	yaml := `apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: broken
  namespace: production
spec:
  refreshInterval: 1h
  target:
    name: db-credentials
  data:
    - secretKey: password
      remoteRef:
        key: prod/db/password
`
	err := ValidateExternalSecretRef([]byte(yaml))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "secretStoreRef")
}

func TestValidateSecretStructure_NoData(t *testing.T) {
	yaml := `apiVersion: v1
kind: Secret
metadata:
  name: empty-secret
type: Opaque
`
	err := ValidateSecretStructure([]byte(yaml))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "data")
}
