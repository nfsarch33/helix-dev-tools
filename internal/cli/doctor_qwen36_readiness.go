package cli

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const qwen36ExpectedBytes int64 = 20467235944

type qwen36CellSpec struct {
	CellID        string
	Status        string
	ModelDir      string
	ExpectedBytes int64
	GPUEnv        string
	Port          int
	MinFreeMIB    int
}

var qwen36Cells = map[string]qwen36CellSpec{
	"C1": {CellID: "C1", Status: "ready", ModelDir: "/mnt/f/models/Qwen3.6-27B-AWQ-INT4", ExpectedBytes: 20467235944, GPUEnv: "QWEN36_C1_VISIBLE_DEVICES", Port: 8004, MinFreeMIB: 4096},
	"C2": {CellID: "C2", Status: "ready", ModelDir: "/mnt/f/models/qwen36-gguf/Qwen3.6-27B-Q4_K_M.gguf", ExpectedBytes: 16817244384, GPUEnv: "QWEN36_C2_VISIBLE_DEVICES", Port: 8005, MinFreeMIB: 4096},
	"C3": {CellID: "C3", Status: "metadata_blocked", ModelDir: "/mnt/f/models/qwen36-gguf/Qwen3.6-14B-Q4_K_M.gguf", GPUEnv: "QWEN36_C3_VISIBLE_DEVICES", Port: 8006, MinFreeMIB: 3072},
	"C4": {CellID: "C4", Status: "metadata_blocked", ModelDir: "/mnt/f/models/qwen36-gguf/Qwen3.6-9B-Q4_K_M.gguf", GPUEnv: "QWEN36_C4_VISIBLE_DEVICES", Port: 8007, MinFreeMIB: 3072},
	"C5": {CellID: "C5", Status: "metadata_blocked", ModelDir: "/mnt/f/models/qwen36-gguf/Qwen3.6-4B-Q4_K_M.gguf", GPUEnv: "QWEN36_C5_VISIBLE_DEVICES", Port: 8008, MinFreeMIB: 1536},
	"C6": {CellID: "C6", Status: "metadata_blocked", ModelDir: "/mnt/f/models/qwen36-gguf/Qwen3.6-8B-Q3_K_M.gguf", GPUEnv: "QWEN36_C6_VISIBLE_DEVICES", Port: 8009, MinFreeMIB: 1536},
}

type qwen36ReadinessState struct {
	CellID             string   `json:"cell_id,omitempty"`
	Status             string   `json:"status,omitempty"`
	ModelDir           string   `json:"model_dir"`
	ModelBytes         int64    `json:"model_bytes"`
	ExpectedModelBytes int64    `json:"expected_model_bytes"`
	GPUDeviceID        string   `json:"gpu_device_id"`
	DockerDeviceIDs    []string `json:"docker_device_ids"`
	FreeMIB            int      `json:"free_mib,omitempty"`
	MinFreeMIB         int      `json:"min_free_mib,omitempty"`
	Port               int      `json:"port"`
	PortAvailable      bool     `json:"port_available"`
}

var qwen36ReadinessFormat string
var qwen36ReadinessCell string

var doctorQwen36ReadinessCmd = &cobra.Command{
	Use:   "qwen36-readiness",
	Short: "Verify Qwen3.6 eval-lane model, GPU pinning, and port readiness",
	RunE: func(cmd *cobra.Command, _ []string) error {
		state := gatherQwen36ReadinessState()
		failures := evaluateQwen36Readiness(state)
		if qwen36ReadinessFormat == "json" || doctorOutputJSON {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(struct {
				State    qwen36ReadinessState `json:"state"`
				Failures []string             `json:"failures"`
				OK       bool                 `json:"ok"`
			}{
				State:    state,
				Failures: failures,
				OK:       len(failures) == 0,
			})
		}
		for _, failure := range failures {
			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "FAIL "+failure)
		}
		if len(failures) > 0 {
			return fmt.Errorf("qwen36-readiness failed: %d failure(s)", len(failures))
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "PASS qwen36-readiness")
		return nil
	},
}

func init() {
	doctorQwen36ReadinessCmd.Flags().StringVar(&qwen36ReadinessFormat, "format", "", "Output format: json")
	doctorQwen36ReadinessCmd.Flags().StringVar(&qwen36ReadinessCell, "cell", "", "Qwen3.6 matrix cell id (C1..C6)")
	doctorCmd.AddCommand(doctorQwen36ReadinessCmd)
}

func evaluateQwen36Readiness(state qwen36ReadinessState) []string {
	var failures []string
	status := strings.TrimSpace(state.Status)
	if status != "" && status != "ready" {
		return append(failures, fmt.Sprintf("cell %s is not runnable: status=%s", state.CellID, status))
	}
	if strings.TrimSpace(state.ModelDir) == "" {
		failures = append(failures, "VLLM_36_MODEL_DIR must point to the locked Qwen3.6 Int4 artefact directory")
	}
	expectedBytes := state.ExpectedModelBytes
	if expectedBytes == 0 {
		expectedBytes = qwen36ExpectedBytes
	}
	if state.ModelBytes != expectedBytes {
		failures = append(failures, fmt.Sprintf("model artefact size mismatch: got %d bytes, want %d bytes", state.ModelBytes, expectedBytes))
	}
	if strings.TrimSpace(state.GPUDeviceID) == "" {
		failures = append(failures, "VLLM_36_VISIBLE_DEVICES must name one RTX 3090 Docker device id")
	}
	if len(state.DockerDeviceIDs) != 1 || state.DockerDeviceIDs[0] != state.GPUDeviceID {
		failures = append(failures, "qwen36-eval must pin exactly one Docker GPU device id matching VLLM_36_VISIBLE_DEVICES")
	}
	if state.MinFreeMIB > 0 && state.FreeMIB < state.MinFreeMIB {
		failures = append(failures, fmt.Sprintf("GPU free memory too low: got %d MiB, want at least %d MiB", state.FreeMIB, state.MinFreeMIB))
	}
	if state.Port <= 0 {
		failures = append(failures, "VLLM_36_HOST_PORT must be a positive TCP port")
	} else if !state.PortAvailable {
		failures = append(failures, fmt.Sprintf("VLLM_36_HOST_PORT %d is already in use", state.Port))
	}
	return failures
}

func gatherQwen36ReadinessState() qwen36ReadinessState {
	if cell := strings.TrimSpace(firstNonEmptyString(os.Getenv("QWEN36_CELL"), qwen36ReadinessCell)); cell != "" {
		spec, ok := qwen36Cells[cell]
		if !ok {
			return qwen36ReadinessState{CellID: cell, Status: "unknown_cell"}
		}
		gpuID := strings.TrimSpace(os.Getenv(spec.GPUEnv))
		freeMIB := intFromEnv("QWEN36_FREE_MIB", gpuFreeMIB(gpuID))
		return qwen36ReadinessState{
			CellID:             spec.CellID,
			Status:             spec.Status,
			ModelDir:           spec.ModelDir,
			ModelBytes:         pathSize(spec.ModelDir),
			ExpectedModelBytes: spec.ExpectedBytes,
			GPUDeviceID:        gpuID,
			DockerDeviceIDs:    splitDeviceIDs(gpuID),
			FreeMIB:            freeMIB,
			MinFreeMIB:         spec.MinFreeMIB,
			Port:               spec.Port,
			PortAvailable:      tcpPortAvailable(spec.Port),
		}
	}
	modelDir := strings.TrimSpace(os.Getenv("VLLM_36_MODEL_DIR"))
	expectedBytes := int64FromEnv("VLLM_36_EXPECTED_BYTES", qwen36ExpectedBytes)
	gpuID := strings.TrimSpace(os.Getenv("VLLM_36_VISIBLE_DEVICES"))
	port := intFromEnv("VLLM_36_HOST_PORT", 8004)
	return qwen36ReadinessState{
		ModelDir:           modelDir,
		ModelBytes:         pathSize(modelDir),
		ExpectedModelBytes: expectedBytes,
		GPUDeviceID:        gpuID,
		DockerDeviceIDs:    splitDeviceIDs(gpuID),
		Port:               port,
		PortAvailable:      tcpPortAvailable(port),
	}
}

func pathSize(root string) int64 {
	if strings.TrimSpace(root) == "" {
		return 0
	}
	info, err := os.Stat(root)
	if err != nil {
		return 0
	}
	if !info.IsDir() {
		return info.Size()
	}
	var total int64
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr == nil {
			for _, part := range strings.Split(rel, string(filepath.Separator)) {
				if part == ".cache" {
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if strings.HasSuffix(name, ".aria2") || strings.HasSuffix(name, ".lock") || strings.HasSuffix(name, ".incomplete") {
			return nil
		}
		info, statErr := d.Info()
		if statErr != nil {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total
}

func gpuFreeMIB(gpuID string) int {
	if strings.TrimSpace(gpuID) == "" {
		return 0
	}
	for _, candidate := range []string{"nvidia-smi", "/usr/lib/wsl/lib/nvidia-smi", "/mnt/c/Windows/System32/nvidia-smi.exe"} {
		out, err := exec.Command(candidate, "--id="+gpuID, "--query-gpu=memory.free", "--format=csv,noheader,nounits").Output()
		if err != nil {
			continue
		}
		value := strings.TrimSpace(string(out))
		if parsed, parseErr := strconv.Atoi(value); parseErr == nil {
			return parsed
		}
	}
	return 0
}

func splitDeviceIDs(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func tcpPortAvailable(port int) bool {
	if port <= 0 {
		return false
	}
	ln, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

func intFromEnv(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func int64FromEnv(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
