package cli

import "testing"

func TestEvaluateQwen36Readiness_AllowsLockedSingleGPULane(t *testing.T) {
	t.Parallel()

	failures := evaluateQwen36Readiness(qwen36ReadinessState{
		CellID:             "C1",
		Status:             "ready",
		ModelDir:           "/mnt/f/models/Qwen3.6-27B-AWQ-INT4",
		ModelBytes:         qwen36ExpectedBytes,
		ExpectedModelBytes: qwen36ExpectedBytes,
		GPUDeviceID:        "GPU-3090-1",
		DockerDeviceIDs:    []string{"GPU-3090-1"},
		FreeMIB:            8192,
		MinFreeMIB:         4096,
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
		Status:             "ready",
		ModelDir:           "",
		ModelBytes:         qwen36ExpectedBytes - 1,
		ExpectedModelBytes: qwen36ExpectedBytes,
		GPUDeviceID:        "GPU-3090-1",
		DockerDeviceIDs:    []string{"GPU-3090-1"},
		FreeMIB:            8192,
		MinFreeMIB:         4096,
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
		Status:             "ready",
		ModelDir:           "/mnt/f/models/Qwen3.6-27B-AWQ-INT4",
		ModelBytes:         qwen36ExpectedBytes,
		ExpectedModelBytes: qwen36ExpectedBytes,
		GPUDeviceID:        "GPU-3090-1",
		DockerDeviceIDs:    []string{"GPU-3090-1", "GPU-3090-2"},
		FreeMIB:            8192,
		MinFreeMIB:         4096,
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
		Status:             "ready",
		ModelDir:           "/mnt/f/models/Qwen3.6-27B-AWQ-INT4",
		ModelBytes:         qwen36ExpectedBytes,
		ExpectedModelBytes: qwen36ExpectedBytes,
		GPUDeviceID:        "GPU-3090-1",
		DockerDeviceIDs:    []string{"GPU-3090-1"},
		FreeMIB:            8192,
		MinFreeMIB:         4096,
		Port:               8004,
		PortAvailable:      false,
	})

	if len(failures) != 1 {
		t.Fatalf("expected port conflict failure, got %d: %v", len(failures), failures)
	}
}

func TestEvaluateQwen36Readiness_RejectsMetadataBlockedCell(t *testing.T) {
	t.Parallel()

	failures := evaluateQwen36Readiness(qwen36ReadinessState{
		CellID:             "C3",
		Status:             "metadata_blocked",
		ModelDir:           "/mnt/f/models/qwen36-gguf/Qwen3.6-14B-Q4_K_M.gguf",
		ModelBytes:         0,
		ExpectedModelBytes: 0,
		GPUDeviceID:        "GPU-4070",
		DockerDeviceIDs:    []string{"GPU-4070"},
		FreeMIB:            12000,
		MinFreeMIB:         3072,
		Port:               8006,
		PortAvailable:      true,
	})

	if len(failures) == 0 {
		t.Fatal("expected metadata-blocked cell failure")
	}
}

func TestEvaluateQwen36Readiness_RejectsLowVRAMHeadroom(t *testing.T) {
	t.Parallel()

	failures := evaluateQwen36Readiness(qwen36ReadinessState{
		CellID:             "C1",
		Status:             "ready",
		ModelDir:           "/mnt/f/models/Qwen3.6-27B-AWQ-INT4",
		ModelBytes:         qwen36ExpectedBytes,
		ExpectedModelBytes: qwen36ExpectedBytes,
		GPUDeviceID:        "GPU-3090-1",
		DockerDeviceIDs:    []string{"GPU-3090-1"},
		FreeMIB:            1024,
		MinFreeMIB:         4096,
		Port:               8004,
		PortAvailable:      true,
	})

	if len(failures) != 1 {
		t.Fatalf("expected low-VRAM failure, got %d: %v", len(failures), failures)
	}
}

func TestQwen36CellManifestContainsWideMatrix(t *testing.T) {
	t.Parallel()

	for _, cell := range []string{"C1", "C2", "C3", "C4", "C5", "C6"} {
		if _, ok := qwen36Cells[cell]; !ok {
			t.Fatalf("missing qwen36 cell %s", cell)
		}
	}
	if qwen36Cells["C1"].ExpectedBytes != qwen36ExpectedBytes {
		t.Fatalf("C1 bytes = %d, want %d", qwen36Cells["C1"].ExpectedBytes, qwen36ExpectedBytes)
	}
	if qwen36Cells["C3"].Status != "metadata_blocked" {
		t.Fatalf("C3 status = %q, want metadata_blocked", qwen36Cells["C3"].Status)
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
