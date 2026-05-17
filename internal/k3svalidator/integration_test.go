package k3svalidator

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func skipUnlessIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv("CURSOR_TOOLS_INTEGRATION") != "1" {
		t.Skip("set CURSOR_TOOLS_INTEGRATION=1 to run integration tests")
	}
}

func TestIntegration_NodeListing(t *testing.T) {
	skipUnlessIntegration(t)

	out, err := exec.Command("kubectl", "get", "nodes", "--no-headers").Output()
	require.NoError(t, err, "kubectl must be reachable")

	header := "NAME   STATUS   ROLES   AGE   VERSION\n"
	nodes, err := ParseNodeStatus(header + string(out))
	require.NoError(t, err)
	assert.NotEmpty(t, nodes, "cluster must have at least one node")

	for _, n := range nodes {
		assert.NotEmpty(t, n.Name)
		assert.NotEmpty(t, n.Version)
	}
}

func TestIntegration_AllNodesReady(t *testing.T) {
	skipUnlessIntegration(t)

	out, err := exec.Command("kubectl", "get", "nodes").Output()
	require.NoError(t, err)

	nodes, err := ParseNodeStatus(string(out))
	require.NoError(t, err)

	err = ValidateAllReady(nodes)
	assert.NoError(t, err, "all nodes should be Ready in a healthy cluster")
}

func TestIntegration_VersionCompatibility(t *testing.T) {
	skipUnlessIntegration(t)

	out, err := exec.Command("kubectl", "get", "nodes").Output()
	require.NoError(t, err)

	nodes, err := ParseNodeStatus(string(out))
	require.NoError(t, err)

	err = CheckVersionCompatibility(nodes)
	assert.NoError(t, err, "node versions should be compatible")
}

func TestIntegration_NamespaceCreateDelete(t *testing.T) {
	skipUnlessIntegration(t)

	ns := "cursor-tools-integration-test"

	out, err := exec.Command("kubectl", "create", "namespace", ns).CombinedOutput()
	require.NoError(t, err, "create ns: %s", string(out))

	defer func() {
		_ = exec.Command("kubectl", "delete", "namespace", ns, "--wait=false").Run()
	}()

	out, err = exec.Command("kubectl", "get", "namespace", ns, "-o", "jsonpath={.status.phase}").Output()
	require.NoError(t, err)
	assert.Equal(t, "Active", string(out))
}

func TestIntegration_ServiceDeployValidation(t *testing.T) {
	skipUnlessIntegration(t)

	ns := "cursor-tools-svc-test"
	_ = exec.Command("kubectl", "create", "namespace", ns).Run()
	defer func() {
		_ = exec.Command("kubectl", "delete", "namespace", ns, "--wait=false").Run()
	}()

	deployYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-test
  namespace: ` + ns + `
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx-test
  template:
    metadata:
      labels:
        app: nginx-test
    spec:
      containers:
      - name: nginx
        image: nginx:alpine
        ports:
        - containerPort: 80`

	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(deployYAML)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "apply deploy: %s", string(out))

	out, err = exec.Command("kubectl", "get", "deployment", "nginx-test", "-n", ns,
		"-o", "jsonpath={.spec.replicas}").Output()
	require.NoError(t, err)
	assert.Equal(t, "1", string(out))
}
