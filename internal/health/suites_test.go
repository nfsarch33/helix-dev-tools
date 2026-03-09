package health_test

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/nfsarch33/cursor-tools/internal/config"
	"github.com/nfsarch33/cursor-tools/internal/health"
)

func TestHealth(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Health Suite")
}

var _ = Describe("BuildAllSuites", func() {
	It("returns 21 suites", func() {
		p := config.DefaultPaths()
		suites := health.BuildAllSuites(p)
		Expect(suites).To(HaveLen(21))
	})

	It("includes DevContainer Compliance as Suite 20", func() {
		p := config.DefaultPaths()
		suites := health.BuildAllSuites(p)
		Expect(suites[19].Name).To(Equal("DevContainer Compliance"))
	})

	It("includes rtk Token Optimization as Suite 21", func() {
		p := config.DefaultPaths()
		suites := health.BuildAllSuites(p)
		Expect(suites[20].Name).To(Equal("rtk Token Optimization"))
	})
})

var _ = Describe("Suite 20: DevContainer Compliance", func() {
	var tmpDir string
	var p config.Paths

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "health-dc-*")
		Expect(err).NotTo(HaveOccurred())

		p = config.Paths{
			Home:     tmpDir,
			GlobalKB: filepath.Join(tmpDir, "Code", "global-kb"),
			Memo:     filepath.Join(tmpDir, "memo"),
			BinDir:   filepath.Join(tmpDir, "bin"),
		}

		ccDir := p.CursorConfigDir()
		ctDir := filepath.Join(ccDir, "cursor-tools")

		os.MkdirAll(filepath.Join(ccDir, "devcontainer-templates", "go-workspace"), 0o755)
		os.MkdirAll(filepath.Join(ctDir, ".devcontainer"), 0o755)
		os.MkdirAll(filepath.Join(ctDir, "build", "package"), 0o755)

		os.WriteFile(filepath.Join(ccDir, "devcontainer-templates", "go-workspace", "Dockerfile"), []byte("FROM golang:1.24\n"), 0o644)
		os.WriteFile(filepath.Join(ccDir, "devcontainer-templates", "go-workspace", "devcontainer.json"), []byte("{}\n"), 0o644)
		os.WriteFile(filepath.Join(ctDir, ".devcontainer", "devcontainer.json"), []byte("{}\n"), 0o644)
		os.WriteFile(filepath.Join(ctDir, "build", "package", "Dockerfile"), []byte("FROM golang\n"), 0o644)
		os.WriteFile(filepath.Join(ctDir, "build", "package", "Dockerfile.dev"), []byte("FROM golang\n"), 0o644)
		os.WriteFile(filepath.Join(ctDir, "Makefile"), []byte("docker-native:\n\techo ok\ntest-docker:\n\techo ok\n"), 0o644)
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	It("produces 8 assertions", func() {
		suites := health.BuildAllSuites(p)
		var dc *health.Suite
		for _, s := range suites {
			if s.Name == "DevContainer Compliance" {
				dc = s
				break
			}
		}
		Expect(dc).NotTo(BeNil())
		Expect(dc.Total()).To(Equal(8))
	})

	It("all 8 assertions pass on a populated fixture", func() {
		suites := health.BuildAllSuites(p)
		var dc *health.Suite
		for _, s := range suites {
			if s.Name == "DevContainer Compliance" {
				dc = s
				break
			}
		}
		Expect(dc).NotTo(BeNil())
		Expect(dc.PassCount()).To(Equal(8))
	})

	It("fails when devcontainer-templates is missing", func() {
		os.RemoveAll(filepath.Join(p.CursorConfigDir(), "devcontainer-templates"))
		suites := health.BuildAllSuites(p)
		var dc *health.Suite
		for _, s := range suites {
			if s.Name == "DevContainer Compliance" {
				dc = s
				break
			}
		}
		Expect(dc).NotTo(BeNil())
		Expect(dc.PassCount()).To(BeNumerically("<", dc.Total()))
	})

	It("fails when test-docker target is missing from Makefile", func() {
		ctDir := filepath.Join(p.CursorConfigDir(), "cursor-tools")
		os.WriteFile(filepath.Join(ctDir, "Makefile"), []byte("docker-native:\n\techo ok\n"), 0o644)
		suites := health.BuildAllSuites(p)
		var dc *health.Suite
		for _, s := range suites {
			if s.Name == "DevContainer Compliance" {
				dc = s
				break
			}
		}
		Expect(dc).NotTo(BeNil())
		Expect(dc.PassCount()).To(Equal(7))
	})
})

var _ = Describe("Suite 21: rtk Token Optimization", func() {
	var tmpDir string
	var p config.Paths

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "health-rtk-*")
		Expect(err).NotTo(HaveOccurred())

		rulesDir := filepath.Join(tmpDir, ".cursor", "rules")
		skillsDir := filepath.Join(tmpDir, ".cursor", "skills")

		p = config.Paths{
			Home:      tmpDir,
			GlobalKB:  filepath.Join(tmpDir, "Code", "global-kb"),
			Memo:      filepath.Join(tmpDir, "memo"),
			BinDir:    filepath.Join(tmpDir, "bin"),
			RulesDir:  rulesDir,
			SkillsDir: skillsDir,
		}

		os.MkdirAll(rulesDir, 0o755)
		os.WriteFile(filepath.Join(rulesDir, "rtk-token-optimization.md"), []byte("---\nalwaysApply: true\n---\n# rtk Token Optimization\nrtk git status\n"), 0o644)

		skillDir := filepath.Join(skillsDir, "rtk-integration")
		os.MkdirAll(skillDir, 0o755)
		os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: rtk-integration\n---\n# rtk Integration\n"), 0o644)
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	It("produces 3 assertions", func() {
		suites := health.BuildAllSuites(p)
		var rtk *health.Suite
		for _, s := range suites {
			if s.Name == "rtk Token Optimization" {
				rtk = s
				break
			}
		}
		Expect(rtk).NotTo(BeNil())
		Expect(rtk.Total()).To(Equal(3))
	})

	It("all pass on a populated fixture", func() {
		suites := health.BuildAllSuites(p)
		var rtk *health.Suite
		for _, s := range suites {
			if s.Name == "rtk Token Optimization" {
				rtk = s
				break
			}
		}
		Expect(rtk).NotTo(BeNil())
		Expect(rtk.PassCount()).To(Equal(3))
	})

	It("fails when rtk rule is missing", func() {
		os.Remove(filepath.Join(p.RulesDir, "rtk-token-optimization.md"))
		suites := health.BuildAllSuites(p)
		var rtk *health.Suite
		for _, s := range suites {
			if s.Name == "rtk Token Optimization" {
				rtk = s
				break
			}
		}
		Expect(rtk).NotTo(BeNil())
		Expect(rtk.PassCount()).To(Equal(2))
	})

	It("fails when rtk skill is missing", func() {
		os.RemoveAll(filepath.Join(p.SkillsDir, "rtk-integration"))
		suites := health.BuildAllSuites(p)
		var rtk *health.Suite
		for _, s := range suites {
			if s.Name == "rtk Token Optimization" {
				rtk = s
				break
			}
		}
		Expect(rtk).NotTo(BeNil())
		Expect(rtk.PassCount()).To(Equal(2))
	})
})
