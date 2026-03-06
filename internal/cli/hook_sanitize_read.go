package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/cursor-tools/internal/config"
	"github.com/nfsarch33/cursor-tools/internal/hookio"
	"github.com/nfsarch33/cursor-tools/internal/logger"
	"github.com/nfsarch33/cursor-tools/internal/patterns"
)

var sanitizeReadCmd = &cobra.Command{
	Use:   "sanitize-read",
	Short: "beforeReadFile: block secret file reads",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSanitizeRead(os.Stdin, os.Stdout)
	},
}

type sanitizeReadHandler struct {
	log *logger.Logger
}

func (h *sanitizeReadHandler) Handle(_ context.Context, input *hookio.Input) (*hookio.Response, error) {
	if input.FilePath == "" {
		return hookio.Allow(), nil
	}

	basename := filepath.Base(input.FilePath)

	for _, blocked := range patterns.BlockedFilenames {
		if basename == blocked {
			h.log.Log(fmt.Sprintf("BLOCKED file=%q match=%q", input.FilePath, blocked))
			return hookio.Deny(
				fmt.Sprintf("BLOCKED: '%s' likely contains secrets", basename),
				fmt.Sprintf("File '%s' was blocked by sanitize-read because it likely contains secrets. Never read secret files.", basename),
			), nil
		}
	}

	if patterns.ContainsAny(input.FilePath, patterns.BlockedDirs) {
		h.log.Log(fmt.Sprintf("BLOCKED file=%q dir=secrets", input.FilePath))
		return hookio.Deny(
			fmt.Sprintf("BLOCKED: path contains secrets directory"),
			fmt.Sprintf("Path '%s' is in a secrets directory and was blocked. Do not access secret directories.", input.FilePath),
		), nil
	}

	for _, ext := range patterns.BlockedExtensions {
		if strings.HasSuffix(strings.ToLower(basename), ext) {
			h.log.Log(fmt.Sprintf("BLOCKED file=%q ext=%q", input.FilePath, ext))
			return hookio.Deny(
				fmt.Sprintf("BLOCKED: '%s' is a key/certificate file", basename),
				"Key and certificate files are blocked by sanitize-read.",
			), nil
		}
	}

	return hookio.Allow(), nil
}

func runSanitizeRead(stdin *os.File, stdout *os.File) error {
	paths := config.DefaultPaths()
	handler := &sanitizeReadHandler{log: logger.New(paths.LogFile("sanitize-read"))}

	input, err := hookio.ReadInput(stdin)
	if err != nil {
		_ = hookio.WriteResponse(stdout, hookio.Allow())
		return nil
	}

	resp, err := handler.Handle(context.Background(), input)
	if err != nil {
		_ = hookio.WriteResponse(stdout, hookio.Allow())
		return nil
	}

	_ = hookio.WriteResponse(stdout, resp)
	if resp.Permission == "deny" {
		os.Exit(2)
	}
	return nil
}
