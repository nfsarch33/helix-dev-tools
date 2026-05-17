package dashboardvalidator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validDashboardDeploymentYAML = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: kubernetes-dashboard-web
  namespace: kubernetes-dashboard
spec:
  replicas: 1
  selector:
    matchLabels:
      app: kubernetes-dashboard-web
  template:
    metadata:
      labels:
        app: kubernetes-dashboard-web
    spec:
      containers:
        - name: dashboard-web
          image: kubernetesui/dashboard-web:1.6.0
          ports:
            - containerPort: 8000
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 250m
              memory: 256Mi
          readinessProbe:
            httpGet:
              path: /
              port: 8000
            initialDelaySeconds: 10
            periodSeconds: 10
`

const validDashboardAPIDeploymentYAML = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: kubernetes-dashboard-api
  namespace: kubernetes-dashboard
spec:
  replicas: 1
  selector:
    matchLabels:
      app: kubernetes-dashboard-api
  template:
    metadata:
      labels:
        app: kubernetes-dashboard-api
    spec:
      containers:
        - name: dashboard-api
          image: kubernetesui/dashboard-api:1.2.0
          ports:
            - containerPort: 9000
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 250m
              memory: 256Mi
          readinessProbe:
            httpGet:
              path: /api/v1/login
              port: 9000
            initialDelaySeconds: 10
            periodSeconds: 10
`

func TestParseDashboardVersion(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		want    string
		wantErr bool
	}{
		{
			name: "v7.x web component",
			yaml: validDashboardDeploymentYAML,
			want: "1.6.0",
		},
		{
			name: "v7.x api component",
			yaml: validDashboardAPIDeploymentYAML,
			want: "1.2.0",
		},
		{
			name:    "no container image",
			yaml:    `apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: foo`,
			wantErr: true,
		},
		{
			name: "legacy v2 single image",
			yaml: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: kubernetes-dashboard
spec:
  template:
    spec:
      containers:
        - name: dashboard
          image: kubernetesui/dashboard:v2.7.0
`,
			want: "v2.7.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDashboardVersion([]byte(tt.yaml))
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidateVersionCompatibility_Pass(t *testing.T) {
	tests := []struct {
		name             string
		dashboardVersion string
		k3sVersion       string
	}{
		{"web 1.6.0 with k3s 1.35", "1.6.0", "v1.35.4+k3s1"},
		{"api 1.2.0 with k3s 1.35", "1.2.0", "v1.35.4+k3s1"},
		{"web 1.5.0 with k3s 1.30", "1.5.0", "v1.30.2+k3s1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVersionCompatibility(tt.dashboardVersion, tt.k3sVersion)
			assert.NoError(t, err)
		})
	}
}

func TestValidateVersionCompatibility_Fail(t *testing.T) {
	tests := []struct {
		name             string
		dashboardVersion string
		k3sVersion       string
	}{
		{"legacy v2.7.0 with k3s 1.35", "v2.7.0", "v1.35.4+k3s1"},
		{"legacy v2.6.0 with k3s 1.32", "v2.6.0", "v1.32.0+k3s1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVersionCompatibility(tt.dashboardVersion, tt.k3sVersion)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "incompatible")
		})
	}
}

func TestValidateDashboardPod(t *testing.T) {
	t.Run("valid pod with limits and probe", func(t *testing.T) {
		err := ValidateDashboardPod([]byte(validDashboardDeploymentYAML))
		assert.NoError(t, err)
	})

	t.Run("missing resource limits", func(t *testing.T) {
		yaml := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: kubernetes-dashboard-web
  namespace: kubernetes-dashboard
spec:
  template:
    spec:
      containers:
        - name: dashboard-web
          image: kubernetesui/dashboard-web:1.6.0
          ports:
            - containerPort: 8000
`
		err := ValidateDashboardPod([]byte(yaml))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "resource limits")
	})

	t.Run("missing readiness probe", func(t *testing.T) {
		yaml := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: kubernetes-dashboard-web
  namespace: kubernetes-dashboard
spec:
  template:
    spec:
      containers:
        - name: dashboard-web
          image: kubernetesui/dashboard-web:1.6.0
          resources:
            limits:
              cpu: 250m
              memory: 256Mi
`
		err := ValidateDashboardPod([]byte(yaml))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "readiness probe")
	})
}

const validRBACYAML = `apiVersion: v1
kind: ServiceAccount
metadata:
  name: kubernetes-dashboard
  namespace: kubernetes-dashboard
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kubernetes-dashboard
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
  - kind: ServiceAccount
    name: kubernetes-dashboard
    namespace: kubernetes-dashboard
`

func TestValidateRBACConfig(t *testing.T) {
	t.Run("valid RBAC with SA and binding", func(t *testing.T) {
		err := ValidateRBACConfig([]byte(validRBACYAML))
		assert.NoError(t, err)
	})

	t.Run("missing service account", func(t *testing.T) {
		yaml := `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kubernetes-dashboard
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
  - kind: ServiceAccount
    name: kubernetes-dashboard
    namespace: kubernetes-dashboard
`
		err := ValidateRBACConfig([]byte(yaml))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ServiceAccount")
	})

	t.Run("missing cluster role binding", func(t *testing.T) {
		yaml := `apiVersion: v1
kind: ServiceAccount
metadata:
  name: kubernetes-dashboard
  namespace: kubernetes-dashboard
`
		err := ValidateRBACConfig([]byte(yaml))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ClusterRoleBinding")
	})
}

const validServiceYAML = `apiVersion: v1
kind: Service
metadata:
  name: kubernetes-dashboard
  namespace: kubernetes-dashboard
spec:
  type: NodePort
  ports:
    - port: 443
      targetPort: 8000
      nodePort: 30043
  selector:
    app: kubernetes-dashboard-web
`

func TestValidateNodePortAccess(t *testing.T) {
	t.Run("valid NodePort 30043", func(t *testing.T) {
		err := ValidateNodePortAccess([]byte(validServiceYAML), 30043)
		assert.NoError(t, err)
	})

	t.Run("wrong NodePort", func(t *testing.T) {
		err := ValidateNodePortAccess([]byte(validServiceYAML), 30080)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "30043")
	})

	t.Run("not NodePort type", func(t *testing.T) {
		yaml := `apiVersion: v1
kind: Service
metadata:
  name: kubernetes-dashboard
spec:
  type: ClusterIP
  ports:
    - port: 443
      targetPort: 8000
`
		err := ValidateNodePortAccess([]byte(yaml), 30043)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "NodePort")
	})

	t.Run("port out of valid range", func(t *testing.T) {
		yaml := `apiVersion: v1
kind: Service
metadata:
  name: kubernetes-dashboard
spec:
  type: NodePort
  ports:
    - port: 443
      targetPort: 8000
      nodePort: 80
`
		err := ValidateNodePortAccess([]byte(yaml), 80)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "range")
	})
}
