package cli

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("mcp index helpers", func() {
	Describe("loadMCPServers", func() {
		It("parses an MCP config file", func() {
			dir := GinkgoT().TempDir()
			cfg := filepath.Join(dir, "mcp.json")
			Expect(os.WriteFile(cfg, []byte(`{"mcpServers":{"perplexity":{"command":"npx","args":["-y","@perplexity-ai/mcp-server"]}}}`), 0o644)).To(Succeed())

			servers, err := loadMCPServers(cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(servers).To(HaveKey("perplexity"))
			Expect(servers["perplexity"].Command).To(Equal("npx"))
		})
	})

	Describe("redactArgs", func() {
		It("redacts flagged credential values", func() {
			args := []string{"--token", "abc", "--api-key=xyz", "--safe", "keep"}
			Expect(redactArgs(args)).To(Equal([]string{"--token", "***REDACTED***", "--api-key=***REDACTED***", "--safe", "keep"}))
		})
	})

	Describe("renderMCPIndex", func() {
		It("renders env keys and redacts values", func() {
			const tokenValue = "abc123-real-token"
			const envValue = "pplx-super-secret"
			md := renderMCPIndex(map[string]mcpServerSpec{
				"perplexity": {
					Command: "npx",
					Args:    []string{"--token", tokenValue, "@perplexity-ai/mcp-server"},
					Env:     map[string]string{"PERPLEXITY_API_KEY": envValue},
				},
			})
			Expect(md).To(ContainSubstring("### perplexity"))
			Expect(md).To(ContainSubstring("***REDACTED***"))
			Expect(md).To(ContainSubstring("PERPLEXITY_API_KEY"))
			Expect(md).NotTo(ContainSubstring(tokenValue))
			Expect(md).NotTo(ContainSubstring(envValue))
		})
	})

	Describe("stripTimestamp", func() {
		It("removes the Last generated line only", func() {
			in := "# Title\nLast generated: 2026-03-10T00:00:00Z\nServer count: 1\n"
			Expect(stripTimestamp(in)).To(Equal("# Title\nServer count: 1\n"))
		})
	})

	Describe("refreshMCPIndex", func() {
		It("writes the index and returns unchanged on second run", func() {
			dir := GinkgoT().TempDir()
			cfg := filepath.Join(dir, "mcp.json")
			out := filepath.Join(dir, "global-memories", "mcp-index-and-selection-sop.md")
			Expect(os.WriteFile(cfg, []byte(`{"mcpServers":{"perplexity":{"command":"npx","args":["-y","@perplexity-ai/mcp-server"]}}}`), 0o644)).To(Succeed())

			updated, err := refreshMCPIndex(cfg, out)
			Expect(err).NotTo(HaveOccurred())
			Expect(updated).To(BeTrue())
			Expect(out).To(BeAnExistingFile())

			updated, err = refreshMCPIndex(cfg, out)
			Expect(err).NotTo(HaveOccurred())
			Expect(updated).To(BeFalse())
		})
	})

	Describe("runMCPIndex", func() {
		It("executes successfully with explicit flags", func() {
			dir := GinkgoT().TempDir()
			cfg := filepath.Join(dir, "mcp.json")
			out := filepath.Join(dir, "pepper", "mcp-index-and-selection-sop.md")
			Expect(os.WriteFile(cfg, []byte(`{"mcpServers":{"perplexity":{"command":"npx","args":["-y","@perplexity-ai/mcp-server"]}}}`), 0o644)).To(Succeed())

			oldJSON := mcpIndexFlags.mcpJSON
			oldOut := mcpIndexFlags.out
			defer func() {
				mcpIndexFlags.mcpJSON = oldJSON
				mcpIndexFlags.out = oldOut
			}()
			mcpIndexFlags.mcpJSON = cfg
			mcpIndexFlags.out = out

			Expect(runMCPIndex(nil, nil)).To(Succeed())
			Expect(out).To(BeAnExistingFile())
		})
	})
})
