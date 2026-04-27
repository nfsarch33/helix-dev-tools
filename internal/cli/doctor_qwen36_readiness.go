package cli

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const qwen36ExpectedBytes int64 = 20444234434

type qwen36ReadinessState struct {
	ModelDir           string   `json:"model_dir"`
	ModelBytes         int64    `json:"model_bytes"`
	ExpectedModelBytes int64    `json:"expected_model_bytes"`
	GPUDeviceID        string   `json:"gpu_device_id"`
	DockerDeviceIDs    []string `json:"docker_device_ids"`
	Port               int      `json:"port"`
	PortAvailable      bool     `json:"port_available"`
}

var qwen36ReadinessFormat string

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
	doctorCmd.AddCommand(doctorQwen36ReadinessCmd)
}

func evaluateQwen36Readiness(state qwen36ReadinessState) []string {
	var failures []string
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
	if state.Port <= 0 {
		failures = append(failures, "VLLM_36_HOST_PORT must be a positive TCP port")
	} else if !state.PortAvailable {
		failures = append(failures, fmt.Sprintf("VLLM_36_HOST_PORT %d is already in use", state.Port))
	}
	return failures
}

func gatherQwen36ReadinessState() qwen36ReadinessState {
	modelDir := strings.TrimSpace(os.Getenv("VLLM_36_MODEL_DIR"))
	expectedBytes := int64FromEnv("VLLM_36_EXPECTED_BYTES", qwen36ExpectedBytes)
	gpuID := strings.TrimSpace(os.Getenv("VLLM_36_VISIBLE_DEVICES"))
	port := intFromEnv("VLLM_36_HOST_PORT", 8004)
	return qwen36ReadinessState{
		ModelDir:           modelDir,
		ModelBytes:         dirSize(modelDir),
		ExpectedModelBytes: expectedBytes,
		GPUDeviceID:        gpuID,
		DockerDeviceIDs:    splitDeviceIDs(gpuID),
		Port:               port,
		PortAvailable:      tcpPortAvailable(port),
	}
}

func dirSize(root string) int64 {
	if strings.TrimSpace(root) == "" {
		return 0
	}
	var total int64
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
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
