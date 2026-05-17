package deployvalidator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const validRollingUpdateYAML = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web-app
  namespace: production
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
  template:
    spec:
      containers:
        - name: web
          image: app:v1.2.3
          imagePullPolicy: IfNotPresent
`

func TestRollingUpdate_Valid(t *testing.T) {
	err := ValidateRollingUpdate([]byte(validRollingUpdateYAML))
	assert.NoError(t, err)
}

func TestRollingUpdate_TooAggressive(t *testing.T) {
	yaml := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web-app
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 5
      maxUnavailable: 3
  template:
    spec:
      containers:
        - name: web
          image: app:v1.2.3
`
	err := ValidateRollingUpdate([]byte(yaml))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maxUnavailable")
}

func TestRollingUpdate_ZeroMaxUnavailable(t *testing.T) {
	yaml := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web-app
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
  template:
    spec:
      containers:
        - name: web
          image: app:v1.2.3
`
	err := ValidateRollingUpdate([]byte(yaml))
	assert.NoError(t, err)
}

const validCanaryConfigYAML = `strategy: canary
weight: 10
steps:
  - setWeight: 10
  - pause:
      duration: 5m
  - setWeight: 50
  - pause:
      duration: 10m
  - setWeight: 100
analysis:
  successRate: 99
  latencyP99: 500ms
`

func TestCanary_Valid(t *testing.T) {
	err := ValidateCanaryConfig([]byte(validCanaryConfigYAML))
	assert.NoError(t, err)
}

func TestCanary_MissingAnalysis(t *testing.T) {
	yaml := `strategy: canary
weight: 10
steps:
  - setWeight: 10
  - pause:
      duration: 5m
  - setWeight: 100
`
	err := ValidateCanaryConfig([]byte(yaml))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "analysis")
}

const validRollbackYAML = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web-app
spec:
  revisionHistoryLimit: 10
  strategy:
    type: RollingUpdate
  template:
    spec:
      containers:
        - name: web
          image: app:v1.2.3
`

func TestRollback_Valid(t *testing.T) {
	err := ValidateRollbackPolicy([]byte(validRollbackYAML))
	assert.NoError(t, err)
}

func TestRollback_NoHistoryLimit(t *testing.T) {
	yaml := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web-app
spec:
  strategy:
    type: RollingUpdate
  template:
    spec:
      containers:
        - name: web
          image: app:v1.2.3
`
	err := ValidateRollbackPolicy([]byte(yaml))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "revisionHistoryLimit")
}

func TestImagePull_IfNotPresent(t *testing.T) {
	err := ValidateImagePullPolicy([]byte(validRollingUpdateYAML))
	assert.NoError(t, err)
}

func TestImagePull_Always_Prod_Fail(t *testing.T) {
	yaml := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web-app
  namespace: production
spec:
  template:
    spec:
      containers:
        - name: web
          image: app:v1.2.3
          imagePullPolicy: Always
`
	err := ValidateImagePullPolicy([]byte(yaml))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "IfNotPresent")
}

func TestStrategy_Rolling(t *testing.T) {
	err := ValidateDeploymentStrategy([]byte(validRollingUpdateYAML), "RollingUpdate")
	assert.NoError(t, err)
}

func TestStrategy_Recreate(t *testing.T) {
	yaml := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web-app
spec:
  strategy:
    type: Recreate
  template:
    spec:
      containers:
        - name: web
          image: app:v1.2.3
`
	err := ValidateDeploymentStrategy([]byte(yaml), "Recreate")
	assert.NoError(t, err)
}

func TestStrategy_Canary(t *testing.T) {
	yaml := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web-app
spec:
  strategy:
    type: RollingUpdate
  template:
    spec:
      containers:
        - name: web
          image: app:v1.2.3
`
	err := ValidateDeploymentStrategy([]byte(yaml), "Canary")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Canary")
}
