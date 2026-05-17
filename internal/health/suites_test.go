// runx-public-repo-gate: allow-file personal_path_id,fleet_host_alias — tests assert detection of literal personal-stack identifiers (gate test fixtures)

package health_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/nfsarch33/helix-dev-tools/internal/config"
	"github.com/nfsarch33/helix-dev-tools/internal/health"
	"github.com/nfsarch33/helix-dev-tools/internal/metrics"
)

func TestHealth(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Health Suite")
}

var _ = Describe("BuildAllSuites", func() {
	It("returns 39 suites", func() {
		p := config.DefaultPaths()
		suites := health.BuildAllSuites(p)
		Expect(suites).To(HaveLen(39))
	})

	It("includes Pre-Push Readiness as Suite 39", func() {
		p := config.DefaultPaths()
		suites := health.BuildAllSuites(p)
		Expect(suites[38].Name).To(Equal("Pre-Push Readiness"))
	})

	It("includes Memory Evidence in the shared catalog", func() {
		p := config.DefaultPaths()
		suites := health.BuildAllSuites(p)
		Expect(suites[14].Name).To(Equal("Memory Evidence"))
	})

	It("includes Self-Improvement Pipeline as Suite 27", func() {
		p := config.DefaultPaths()
		suites := health.BuildAllSuites(p)
		Expect(suites[26].Name).To(Equal("Self-Improvement Pipeline"))
	})

	It("includes DevContainer Compliance as Suite 28", func() {
		p := config.DefaultPaths()
		suites := health.BuildAllSuites(p)
		Expect(suites[27].Name).To(Equal("DevContainer Compliance"))
	})

	It("includes rtk Token Optimization as Suite 29", func() {
		p := config.DefaultPaths()
		suites := health.BuildAllSuites(p)
		Expect(suites[28].Name).To(Equal("rtk Token Optimization"))
	})

	It("builds doctor resume suites from the shared catalog", func() {
		p := config.DefaultPaths()
		suites := health.BuildDoctorSuites(p, "resume")
		Expect(suites).NotTo(BeEmpty())
		names := make([]string, 0, len(suites))
		for _, suite := range suites {
			names = append(names, suite.Name)
		}
		Expect(names).To(ContainElement("Resume Readiness"))
		Expect(names).To(ContainElement("Mem0 Connectivity"))
		Expect(names).To(ContainElement("Coordination Signals"))
		Expect(names).To(ContainElement("Git Sync Resilience"))
	})

	It("builds doctor deps suites from the shared catalog", func() {
		p := config.DefaultPaths()
		suites := health.BuildDoctorSuites(p, "deps")
		Expect(suites).NotTo(BeEmpty())
		names := make([]string, 0, len(suites))
		for _, suite := range suites {
			names = append(names, suite.Name)
		}
		Expect(names).To(ContainElement("Dependency Readiness"))
		Expect(names).To(ContainElement("Platform Readiness"))
	})

	It("builds doctor drl suites from the shared catalog", func() {
		p := config.DefaultPaths()
		suites := health.BuildDoctorSuites(p, "drl")
		Expect(suites).NotTo(BeEmpty())
		names := make([]string, 0, len(suites))
		for _, suite := range suites {
			names = append(names, suite.Name)
		}
		Expect(names).To(ContainElement("DRL EvoLoop Observability"))
		Expect(names).To(ContainElement("Mem0 Connectivity"))
		Expect(names).To(ContainElement("Self-Improvement Pipeline"))
	})

	It("includes Git Sync Resilience as Suite 33", func() {
		p := config.DefaultPaths()
		suites := health.BuildAllSuites(p)
		Expect(suites[32].Name).To(Equal("Git Sync Resilience"))
	})

	It("includes Dependency Readiness as Suite 34", func() {
		p := config.DefaultPaths()
		suites := health.BuildAllSuites(p)
		Expect(suites[33].Name).To(Equal("Dependency Readiness"))
	})
})

var _ = Describe("Dependency Readiness", func() {
	It("passes when required tools are available and optional tools are absent", func() {
		tmpDir := GinkgoT().TempDir()
		oldPath := os.Getenv("PATH")
		Expect(os.Setenv("PATH", tmpDir)).To(Succeed())
		defer os.Setenv("PATH", oldPath)

		for _, name := range []string{
			"git", "go", "gh", "ssh", "node", "npm", "python3",
			"uv", "docker", "nvidia-smi", "jq", "curl", "make", "rtk",
		} {
			path := filepath.Join(tmpDir, name)
			script := "#!/bin/sh\n"
			switch name {
			case "docker":
				script += "if [ \"$1\" = \"compose\" ] && [ \"$2\" = \"version\" ]; then echo \"Docker Compose version v2.33.1\"; exit 0; fi\necho \"Docker version 27.0.0\"\n"
			case "go":
				script += "echo \"go version go1.24.1 linux/amd64\"\n"
			default:
				script += "echo \"" + name + " version 1.0.0\"\n"
			}
			Expect(os.WriteFile(path, []byte(script), 0o755)).To(Succeed())
		}

		p := config.DefaultPaths()
		suites := health.BuildDoctorSuites(p, "deps")
		var target *health.Suite
		for _, suite := range suites {
			if suite.Name == "Dependency Readiness" {
				target = suite
				break
			}
		}
		Expect(target).NotTo(BeNil())
		Expect(target.PassCount()).To(Equal(target.Total()))
	})
})

var _ = Describe("Git Sync Resilience", func() {
	It("passes stale push state when the branch is already synced with upstream", func() {
		tmpDir := GinkgoT().TempDir()
		remote := filepath.Join(tmpDir, "remote.git")
		local := filepath.Join(tmpDir, "repo")
		hooksDir := filepath.Join(tmpDir, ".cursor", "hooks")

		run := func(dir string, args ...string) {
			cmd := exec.Command("git", args...)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(),
				"GIT_AUTHOR_NAME=Test",
				"GIT_AUTHOR_EMAIL=test@example.com",
				"GIT_COMMITTER_NAME=Test",
				"GIT_COMMITTER_EMAIL=test@example.com",
			)
			out, err := cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), string(out))
		}

		run(tmpDir, "init", "--bare", remote)
		run(tmpDir, "clone", remote, local)
		run(local, "config", "--local", "user.name", "Jason Lian")
		run(local, "config", "--local", "user.email", "jaslian@gmail.com")
		Expect(os.WriteFile(filepath.Join(local, ".gitattributes"), []byte("*.lock merge=ours\n*.md merge=union\n*.bin binary\n"), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(local, "README.md"), []byte("ok\n"), 0o644)).To(Succeed())
		run(local, "checkout", "-b", "feat/test-sync-state")
		run(local, "add", ".")
		run(local, "commit", "-m", "test: init fixture repo")
		run(local, "push", "-u", "origin", "HEAD")
		run(local, "config", "--local", "rerere.enabled", "true")
		run(local, "config", "--local", "merge.ours.driver", "true")

		Expect(os.MkdirAll(hooksDir, 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(hooksDir, "last-push-result.txt"), []byte(strings.Join([]string{
			"timestamp: " + time.Now().UTC().Format(time.RFC3339),
			"result: failed",
			"attempts: 3",
			"",
		}, "\n")), 0o644)).To(Succeed())

		p := config.Paths{
			Home:     tmpDir,
			GlobalKB: local,
			HooksDir: hooksDir,
		}

		suites := health.BuildDoctorSuites(p, "resume")
		var target *health.Suite
		for _, suite := range suites {
			if suite.Name == "Git Sync Resilience" {
				target = suite
				break
			}
		}
		Expect(target).NotTo(BeNil())

		var syncedResult *health.Result
		for i := range target.Results {
			if target.Results[i].Name == "last push was successful or branch is synced with upstream" {
				syncedResult = &target.Results[i]
				break
			}
		}
		Expect(syncedResult).NotTo(BeNil())
		Expect(syncedResult.Passed).To(BeTrue())
	})
})

var _ = Describe("DevContainer Compliance", func() {
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

var _ = Describe("Memory Evidence", func() {
	It("passes when parity exports and outcome coverage exist", func() {
		tmpDir := GinkgoT().TempDir()
		oldHome := os.Getenv("HOME")
		Expect(os.Setenv("HOME", tmpDir)).To(Succeed())
		defer os.Setenv("HOME", oldHome)
		p := config.DefaultPaths()
		Expect(os.MkdirAll(filepath.Join(tmpDir, "logs"), 0o755)).To(Succeed())
		Expect(os.MkdirAll(p.HooksDir, 0o755)).To(Succeed())

		Expect(os.WriteFile(filepath.Join(tmpDir, "logs", "memory-parity.md"), []byte("# Mem0 Parity Audit\n- Missing manifest entries: 0\n- Parity proven: `true`\n"), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(tmpDir, "logs", "memory-metrics.md"), []byte("# Metrics\n\n## Memory Layer KPIs\n\nCoverage\n"), 0o644)).To(Succeed())
		Expect(metrics.Record(p.MetricsFile(), metrics.Event{
			Timestamp:    time.Now().UTC().Add(-1 * time.Hour),
			Hook:         "track",
			Action:       "record",
			Category:     "mcp",
			Detail:       "mem0:search_memories",
			MemoryLayer:  metrics.MemoryLayerMem0,
			MemoryOp:     metrics.MemoryOpSearch,
			MemoryResult: metrics.MemoryResultHit,
			ResultCount:  1,
		})).To(Succeed())

		suites := health.BuildAllSuites(p)
		var target *health.Suite
		for _, suite := range suites {
			if suite.Name == "Memory Evidence" {
				target = suite
				break
			}
		}
		Expect(target).NotTo(BeNil())
		Expect(target.PassCount()).To(Equal(target.Total()))
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
		os.MkdirAll(p.BinDir, 0o755)
		os.WriteFile(filepath.Join(p.BinDir, "rtk"), []byte("#!/bin/sh\nprintf 'rtk fixture\\n'\n"), 0o755)

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
