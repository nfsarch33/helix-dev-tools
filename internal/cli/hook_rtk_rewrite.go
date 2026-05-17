package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/helix-dev-tools/internal/hookio"
)

var rtkRewriteCmd = &cobra.Command{
	Use:   "rtk-rewrite",
	Short: "PreToolUse: rewrite shell commands through rtk for token savings",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRtkRewriteIO(os.Stdin, os.Stdout)
	},
}

type rtkRewriteHandler struct {
	rtkBin string
}

func rtkBinPath() (string, error) {
	p, err := exec.LookPath("rtk")
	if err != nil {
		return "", fmt.Errorf("rtk not on PATH: %w", err)
	}
	return p, nil
}

func checkRtkVersion(bin string) bool {
	out, err := exec.Command(bin, "--version").Output()
	if err != nil {
		return false
	}
	version := strings.TrimSpace(string(out))
	parts := strings.Split(version, " ")
	for _, p := range parts {
		segs := strings.SplitN(p, ".", 3)
		if len(segs) < 2 {
			continue
		}
		major, e1 := strconv.Atoi(segs[0])
		minor, e2 := strconv.Atoi(segs[1])
		if e1 != nil || e2 != nil {
			continue
		}
		if major > 0 || (major == 0 && minor >= 23) {
			return true
		}
		return false
	}
	return false
}

func (h *rtkRewriteHandler) handle(input *hookio.Input) (interface{}, error) {
	if input.Command == "" {
		return nil, nil
	}

	if _, err := os.Stat(h.rtkBin); err != nil {
		return nil, nil
	}

	if !checkRtkVersion(h.rtkBin) {
		return nil, nil
	}

	out, err := exec.Command(h.rtkBin, "rewrite", input.Command).Output()
	if err != nil {
		return nil, nil
	}

	rewritten := strings.TrimSpace(string(out))
	if rewritten == "" || rewritten == input.Command {
		return nil, nil
	}

	return rtkRewriteResponse(input.Command, rewritten), nil
}

type rtkRewriteOutput struct {
	HookSpecificOutput rtkHookSpec `json:"hookSpecificOutput"`
}

type rtkHookSpec struct {
	HookEventName            string                 `json:"hookEventName"`
	PermissionDecision       string                 `json:"permissionDecision"`
	PermissionDecisionReason string                 `json:"permissionDecisionReason"`
	UpdatedInput             map[string]interface{} `json:"updatedInput"`
}

func rtkRewriteResponse(original, rewritten string) *rtkRewriteOutput {
	return &rtkRewriteOutput{
		HookSpecificOutput: rtkHookSpec{
			HookEventName:            "PreToolUse",
			PermissionDecision:       "allow",
			PermissionDecisionReason: "RTK auto-rewrite",
			UpdatedInput: map[string]interface{}{
				"command": rewritten,
			},
		},
	}
}

func runRtkRewriteIO(stdin io.Reader, stdout io.Writer) error {
	input, err := hookio.ReadInput(stdin)
	if err != nil {
		return nil
	}

	if input.Command == "" {
		return nil
	}

	bin, err := rtkBinPath()
	if err != nil {
		return nil
	}

	h := &rtkRewriteHandler{rtkBin: bin}
	resp, err := h.handle(input)
	if err != nil || resp == nil {
		return nil
	}

	data, err := json.Marshal(resp)
	if err != nil {
		return nil
	}
	_, err = stdout.Write(data)
	return err
}
