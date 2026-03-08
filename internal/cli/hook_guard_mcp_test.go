package cli

import (
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/nfsarch33/cursor-tools/internal/hookio"
	"github.com/nfsarch33/cursor-tools/internal/logger"
)

var _ = Describe("guardMcpHandler", func() {
	var (
		handler     *guardMcpHandler
		metricsFile string
	)

	BeforeEach(func() {
		tmpDir := GinkgoT().TempDir()
		metricsFile = filepath.Join(tmpDir, "metrics.jsonl")
		handler = &guardMcpHandler{
			log:         logger.New(filepath.Join(tmpDir, "test.log")),
			metricsPath: metricsFile,
		}
	})

	It("allows safe MCP tools", func() {
		input := &hookio.Input{ToolName: "read_file", ToolInput: `{"path":"/tmp/test.txt"}`}
		resp, err := handler.Handle(context.Background(), input)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Permission).To(Equal("allow"))
	})

	It("allows empty tool name", func() {
		input := &hookio.Input{ToolName: ""}
		resp, err := handler.Handle(context.Background(), input)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Permission).To(Equal("allow"))
	})

	It("records metrics with bytes_in", func() {
		toolInput := `{"query":"test search"}`
		input := &hookio.Input{ToolName: "search", ToolInput: toolInput}
		_, err := handler.Handle(context.Background(), input)
		Expect(err).NotTo(HaveOccurred())

		data, err := os.ReadFile(metricsFile)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(ContainSubstring("guard-mcp"))
		Expect(string(data)).To(ContainSubstring("bytes_in"))
	})

	It("logs MCP tool invocations", func() {
		input := &hookio.Input{ToolName: "context7_resolve", ToolInput: "some input data"}
		_, err := handler.Handle(context.Background(), input)
		Expect(err).NotTo(HaveOccurred())
	})
})
