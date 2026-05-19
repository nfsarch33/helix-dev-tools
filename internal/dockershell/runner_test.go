package dockershell_test

import (
	"context"
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/dockershell"
	"github.com/stretchr/testify/assert"
)

func TestRunner_Available(t *testing.T) {
	r := dockershell.NewRunner()
	// just verify it doesn't panic; may be false in CI without Docker
	_ = r.Available()
}

func TestRunner_ExecNilConfig(t *testing.T) {
	r := dockershell.NewRunner()
	_, err := r.Exec(context.Background(), nil, "echo", "hello")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nil config")
}

func TestRunner_BuildsCorrectBinary(t *testing.T) {
	r := dockershell.NewRunner()
	assert.NotEmpty(t, r.DockerBin)
}

func TestExecResult_Fields(t *testing.T) {
	r := dockershell.ExecResult{
		ExitCode: 0,
		Stdout:   "hello\n",
		Stderr:   "",
	}
	assert.Equal(t, 0, r.ExitCode)
	assert.Equal(t, "hello\n", r.Stdout)
}
