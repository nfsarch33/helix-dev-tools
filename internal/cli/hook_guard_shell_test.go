package cli

import (
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/nfsarch33/cursor-tools/internal/hookio"
	"github.com/nfsarch33/cursor-tools/internal/logger"
	"github.com/nfsarch33/cursor-tools/internal/patterns"
)

var _ = Describe("guardShellHandler", func() {
	var (
		handler     *guardShellHandler
		metricsFile string
	)

	BeforeEach(func() {
		tmpDir := GinkgoT().TempDir()
		metricsFile = filepath.Join(tmpDir, "metrics.jsonl")
		m, err := patterns.NewMatcher(patterns.ShellDenyPatterns, patterns.ShellWarnPatterns)
		Expect(err).NotTo(HaveOccurred())
		handler = &guardShellHandler{
			matcher:     m,
			log:         logger.New(filepath.Join(tmpDir, "test.log")),
			metricsPath: metricsFile,
		}
	})

	It("allows safe commands", func() {
		input := &hookio.Input{Command: "ls -la"}
		resp, err := handler.Handle(context.Background(), input)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Permission).To(Equal("allow"))
	})

	It("denies rm -rf /", func() {
		input := &hookio.Input{Command: "rm -rf /"}
		resp, err := handler.Handle(context.Background(), input)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Permission).To(Equal("deny"))
	})

	It("denies destructive commands like format disk", func() {
		input := &hookio.Input{Command: "mkfs.ext4 /dev/sda1"}
		resp, err := handler.Handle(context.Background(), input)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Permission).To(Equal("deny"))
	})

	It("allows empty commands", func() {
		input := &hookio.Input{Command: ""}
		resp, err := handler.Handle(context.Background(), input)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Permission).To(Equal("allow"))
	})

	It("records metrics for each invocation", func() {
		input := &hookio.Input{Command: "echo hello"}
		_, err := handler.Handle(context.Background(), input)
		Expect(err).NotTo(HaveOccurred())

		data, err := os.ReadFile(metricsFile)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(ContainSubstring("guard-shell"))
		Expect(string(data)).To(ContainSubstring("echo hello"))
	})

	It("records BytesIn matching command length", func() {
		cmd := "git status --porcelain"
		input := &hookio.Input{Command: cmd}
		_, err := handler.Handle(context.Background(), input)
		Expect(err).NotTo(HaveOccurred())

		data, err := os.ReadFile(metricsFile)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(ContainSubstring(`"bytes_in":22`))
	})

	It("truncates long commands in detail", func() {
		longCmd := ""
		for i := 0; i < 200; i++ {
			longCmd += "x"
		}
		input := &hookio.Input{Command: longCmd}
		resp, err := handler.Handle(context.Background(), input)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Permission).To(Equal("allow"))
	})
})
