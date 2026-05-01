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

		It("renders browser-session MCP safety guidance", func() {
			md := renderMCPIndex(map[string]mcpServerSpec{
				"linkedin-mcp": {Command: "uv"},
				"upwork-mcp":   {Command: "uv"},
			})

			Expect(md).To(ContainSubstring("## Freelancing / job board MCP safety"))
			Expect(md).To(ContainSubstring("Human approval is mandatory"))
			Expect(md).To(ContainSubstring("## Browser-session MCP architecture variants"))
			Expect(md).To(ContainSubstring("`linkedin-mcp` -> `linkedin-job-hunt`"))
			Expect(md).To(ContainSubstring("`upwork-mcp` -> `upwork-job-hunt`"))
		})

		It("preserves Upwork session-inconsistency operational guidance across regens", func() {
			md := renderMCPIndex(map[string]mcpServerSpec{"upwork-mcp": {Command: "uv"}})

			// Past regression: a leaner mcp-index regen wiped the session
			// inconsistency note plus locked-decisions paste-ready reminder.
			// Lock those in here so future reductions trip the test.
			Expect(md).To(ContainSubstring("upwork_check_session"))
			Expect(md).To(ContainSubstring("Not logged in to Upwork"))
			Expect(md).To(ContainSubstring("decisions now locked 2026-05-01T11:55+10:00"))
			Expect(md).To(ContainSubstring("paste-ready copy includes Zendesk + ANZ named"))
			Expect(md).To(ContainSubstring("fix branch is installed"))
		})
	})

	Describe("stripTimestamp", func() {
		It("removes volatile generated and reviewed lines only", func() {
			in := "# Title\nLast generated: 2026-03-10T00:00:00Z\nLast reviewed: 2026-03-10T01:00:00Z\nServer count: 1\n"
			Expect(stripTimestamp(in)).To(Equal("# Title\nServer count: 1"))
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

		It("fails check mode when the index is stale", func() {
			dir := GinkgoT().TempDir()
			cfg := filepath.Join(dir, "mcp.json")
			out := filepath.Join(dir, "pepper", "mcp-index-and-selection-sop.md")
			Expect(os.MkdirAll(filepath.Dir(out), 0o755)).To(Succeed())
			Expect(os.WriteFile(cfg, []byte(`{"mcpServers":{"sentrux":{"command":"sentrux","args":["--mcp"]}}}`), 0o644)).To(Succeed())
			Expect(os.WriteFile(out, []byte("# stale\n"), 0o644)).To(Succeed())

			oldJSON := mcpIndexFlags.mcpJSON
			oldOut := mcpIndexFlags.out
			oldCheck := mcpIndexFlags.check
			oldWrite := mcpIndexFlags.write
			defer func() {
				mcpIndexFlags.mcpJSON = oldJSON
				mcpIndexFlags.out = oldOut
				mcpIndexFlags.check = oldCheck
				mcpIndexFlags.write = oldWrite
			}()
			mcpIndexFlags.mcpJSON = cfg
			mcpIndexFlags.out = out
			mcpIndexFlags.check = true
			mcpIndexFlags.write = false

			Expect(runMCPIndex(nil, nil)).To(MatchError(ContainSubstring("MCP index is stale")))
		})

		It("passes check mode when the index is current", func() {
			dir := GinkgoT().TempDir()
			cfg := filepath.Join(dir, "mcp.json")
			out := filepath.Join(dir, "pepper", "mcp-index-and-selection-sop.md")
			Expect(os.WriteFile(cfg, []byte(`{"mcpServers":{"sentrux":{"command":"sentrux","args":["--mcp"]}}}`), 0o644)).To(Succeed())
			updated, err := refreshMCPIndex(cfg, out)
			Expect(err).NotTo(HaveOccurred())
			Expect(updated).To(BeTrue())

			oldJSON := mcpIndexFlags.mcpJSON
			oldOut := mcpIndexFlags.out
			oldCheck := mcpIndexFlags.check
			oldWrite := mcpIndexFlags.write
			defer func() {
				mcpIndexFlags.mcpJSON = oldJSON
				mcpIndexFlags.out = oldOut
				mcpIndexFlags.check = oldCheck
				mcpIndexFlags.write = oldWrite
			}()
			mcpIndexFlags.mcpJSON = cfg
			mcpIndexFlags.out = out
			mcpIndexFlags.check = true
			mcpIndexFlags.write = false

			Expect(runMCPIndex(nil, nil)).To(Succeed())
		})
	})
})
