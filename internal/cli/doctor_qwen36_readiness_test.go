package cli

import "testing"

func TestEvaluateQwen36Readiness_AllowsLockedSingleGPULane(t *testing.T) {
	t.Parallel()

	failures := evaluateQwen36Readiness(qwen36ReadinessState{
		ModelDir:           "/mnt/f/models/Qwen3.6-27B-AWQ-INT4",
		ModelBytes:         qwen36ExpectedBytes,
		ExpectedModelBytes: qwen36ExpectedBytes,
		GPUDeviceID:        "GPU-3090-1",
		DockerDeviceIDs:    []string{"GPU-3090-1"},
		Port:               8004,
		PortAvailable:      true,
	})

	if len(failures) != 0 {
		t.Fatalf("expected no failures, got %v", failures)
	}
}

func TestEvaluateQwen36Readiness_RejectsMissingModelOrShortArtefact(t *testing.T) {
	t.Parallel()

	failures := evaluateQwen36Readiness(qwen36ReadinessState{
		ModelDir:           "",
		ModelBytes:         qwen36ExpectedBytes - 1,
		ExpectedModelBytes: qwen36ExpectedBytes,
		GPUDeviceID:        "GPU-3090-1",
		DockerDeviceIDs:    []string{"GPU-3090-1"},
		Port:               8004,
		PortAvailable:      true,
	})

	if len(failures) != 2 {
		t.Fatalf("expected model dir and artefact size failures, got %d: %v", len(failures), failures)
	}
}

func TestEvaluateQwen36Readiness_RejectsUnpinnedOrMultiGPUDeviceRequests(t *testing.T) {
	t.Parallel()

	failures := evaluateQwen36Readiness(qwen36ReadinessState{
		ModelDir:           "/mnt/f/models/Qwen3.6-27B-AWQ-INT4",
		ModelBytes:         qwen36ExpectedBytes,
		ExpectedModelBytes: qwen36ExpectedBytes,
		GPUDeviceID:        "GPU-3090-1",
		DockerDeviceIDs:    []string{"GPU-3090-1", "GPU-3090-2"},
		Port:               8004,
		PortAvailable:      true,
	})

	if len(failures) != 1 {
		t.Fatalf("expected single GPU pinning failure, got %d: %v", len(failures), failures)
	}
}

func TestEvaluateQwen36Readiness_RejectsPortConflict(t *testing.T) {
	t.Parallel()

	failures := evaluateQwen36Readiness(qwen36ReadinessState{
		ModelDir:           "/mnt/f/models/Qwen3.6-27B-AWQ-INT4",
		ModelBytes:         qwen36ExpectedBytes,
		ExpectedModelBytes: qwen36ExpectedBytes,
		GPUDeviceID:        "GPU-3090-1",
		DockerDeviceIDs:    []string{"GPU-3090-1"},
		Port:               8004,
		PortAvailable:      false,
	})

	if len(failures) != 1 {
		t.Fatalf("expected port conflict failure, got %d: %v", len(failures), failures)
	}
}

func TestDoctorQwen36ReadinessCommandRegistered(t *testing.T) {
	t.Parallel()

	names := []string{}
	for _, cmd := range doctorCmd.Commands() {
		names = append(names, cmd.Name())
	}
	if !containsString(names, "qwen36-readiness") {
		t.Fatalf("doctor qwen36-readiness command not registered; got %v", names)
	}
}
