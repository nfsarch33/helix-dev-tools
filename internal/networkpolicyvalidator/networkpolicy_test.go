package networkpolicyvalidator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validNetworkPolicyYAML = `apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-web-to-api
  namespace: production
spec:
  podSelector:
    matchLabels:
      app: api
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - podSelector:
            matchLabels:
              app: web
      ports:
        - protocol: TCP
          port: 8080
  egress:
    - to:
        - ipBlock:
            cidr: 10.0.0.0/8
      ports:
        - protocol: TCP
          port: 5432
`

const defaultDenyYAML = `apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny-all
  namespace: production
spec:
  podSelector: {}
  policyTypes:
    - Ingress
    - Egress
`

func TestParseNetworkPolicy_Valid(t *testing.T) {
	np, err := ParseNetworkPolicy([]byte(validNetworkPolicyYAML))
	require.NoError(t, err)
	assert.Equal(t, "allow-web-to-api", np.Metadata.Name)
	assert.Equal(t, "production", np.Metadata.Namespace)
	assert.Contains(t, np.Spec.PolicyTypes, "Ingress")
	assert.Contains(t, np.Spec.PolicyTypes, "Egress")
}

func TestParseNetworkPolicy_Invalid(t *testing.T) {
	_, err := ParseNetworkPolicy([]byte(`not: valid: yaml: [[[`))
	assert.Error(t, err)

	_, err = ParseNetworkPolicy([]byte(`apiVersion: v1
kind: Service
metadata:
  name: not-a-network-policy
`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NetworkPolicy")
}

func TestValidateNamespaceIsolation_DefaultDeny(t *testing.T) {
	deny, err := ParseNetworkPolicy([]byte(defaultDenyYAML))
	require.NoError(t, err)

	allow, err := ParseNetworkPolicy([]byte(validNetworkPolicyYAML))
	require.NoError(t, err)

	err = ValidateNamespaceIsolation([]NetworkPolicy{*deny, *allow}, "production")
	assert.NoError(t, err)
}

func TestValidateNamespaceIsolation_AllowSpecific(t *testing.T) {
	allow, err := ParseNetworkPolicy([]byte(validNetworkPolicyYAML))
	require.NoError(t, err)

	err = ValidateNamespaceIsolation([]NetworkPolicy{*allow}, "production")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "default-deny")
}

func TestValidatePodToPod_Allow(t *testing.T) {
	np, err := ParseNetworkPolicy([]byte(validNetworkPolicyYAML))
	require.NoError(t, err)

	src := map[string]string{"app": "web"}
	dst := map[string]string{"app": "api"}
	err = ValidatePodToPodPolicy(*np, src, dst)
	assert.NoError(t, err)
}

func TestValidatePodToPod_Deny(t *testing.T) {
	np, err := ParseNetworkPolicy([]byte(validNetworkPolicyYAML))
	require.NoError(t, err)

	src := map[string]string{"app": "malicious"}
	dst := map[string]string{"app": "api"}
	err = ValidatePodToPodPolicy(*np, src, dst)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no ingress rule")
}

func TestValidateEgress_AllowCIDR(t *testing.T) {
	np, err := ParseNetworkPolicy([]byte(validNetworkPolicyYAML))
	require.NoError(t, err)

	err = ValidateEgressRules(*np, []string{"10.0.0.0/8"})
	assert.NoError(t, err)
}

func TestValidateEgress_DenyAll(t *testing.T) {
	np, err := ParseNetworkPolicy([]byte(defaultDenyYAML))
	require.NoError(t, err)

	err = ValidateEgressRules(*np, []string{"10.0.0.0/8"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no egress rules")
}

func TestValidateIngress_AllowPort(t *testing.T) {
	np, err := ParseNetworkPolicy([]byte(validNetworkPolicyYAML))
	require.NoError(t, err)

	err = ValidateIngressRules(*np, []int{8080})
	assert.NoError(t, err)
}

func TestValidateIngress_DenyUnlisted(t *testing.T) {
	np, err := ParseNetworkPolicy([]byte(validNetworkPolicyYAML))
	require.NoError(t, err)

	err = ValidateIngressRules(*np, []int{9090})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "9090")
}
