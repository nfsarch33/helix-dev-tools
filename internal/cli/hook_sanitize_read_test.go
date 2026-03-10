package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/nfsarch33/cursor-tools/internal/hookio"
	"github.com/nfsarch33/cursor-tools/internal/logger"
)

var _ = Describe("sanitizeReadHandler", func() {
	var (
		handler     *sanitizeReadHandler
		metricsFile string
	)

	BeforeEach(func() {
		tmpDir := GinkgoT().TempDir()
		metricsFile = filepath.Join(tmpDir, "metrics.jsonl")
		handler = &sanitizeReadHandler{
			log:         logger.New(filepath.Join(tmpDir, "test.log")),
			metricsPath: metricsFile,
		}
	})

	It("allows normal file reads", func() {
		input := &hookio.Input{FilePath: "/Users/test/project/main.go"}
		resp, err := handler.Handle(context.Background(), input)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Permission).To(Equal("allow"))
	})

	It("blocks .env files", func() {
		input := &hookio.Input{FilePath: "/Users/test/project/.env"}
		resp, err := handler.Handle(context.Background(), input)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Permission).To(Equal("deny"))
	})

	It("emits a skill read event when SKILL.md is read", func() {
		input := &hookio.Input{FilePath: "/Users/test/.cursor/skills/rust-mastery/SKILL.md"}
		resp, err := handler.Handle(context.Background(), input)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Permission).To(Equal("allow"))

		data, err := os.ReadFile(metricsFile)
		Expect(err).NotTo(HaveOccurred())

		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		Expect(len(lines)).To(BeNumerically(">=", 2))

		// First line: sanitize-read allow
		Expect(lines[0]).To(ContainSubstring("sanitize-read"))
		// Second line: skill-activate
		Expect(lines[1]).To(ContainSubstring("skill-activate"))
		Expect(lines[1]).To(ContainSubstring("rust-mastery"))
		Expect(lines[1]).To(ContainSubstring(`"cat":"skill"`))
		Expect(lines[1]).To(ContainSubstring(`"action":"read"`))
	})

	It("does not emit skill-activate for non-skill SKILL.md paths", func() {
		input := &hookio.Input{FilePath: "/Users/test/project/SKILL.md"}
		resp, err := handler.Handle(context.Background(), input)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Permission).To(Equal("allow"))

		data, err := os.ReadFile(metricsFile)
		Expect(err).NotTo(HaveOccurred())

		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		Expect(len(lines)).To(Equal(1))
		Expect(lines[0]).NotTo(ContainSubstring("skill-activate"))
	})

	It("extracts correct skill name from nested paths", func() {
		Expect(extractSkillName("/Users/test/.cursor/skills/go-clean-architecture/SKILL.md")).To(Equal("go-clean-architecture"))
		Expect(extractSkillName("/home/user/.agents/skills/mcp-builder/SKILL.md")).To(Equal("mcp-builder"))
	})
})
