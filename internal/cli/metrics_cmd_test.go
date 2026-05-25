// runx-public-repo-gate: allow-file fleet_host_alias,internal_service_id — EvoLoop tests verify machine and source filters using the literal canonical labels

package cli

import (
	"io"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/nfsarch33/helix-dev-tools/internal/config"
	"github.com/nfsarch33/helix-dev-tools/internal/evoloop"
	"github.com/nfsarch33/helix-dev-tools/internal/metrics"
)

var _ = Describe("metrics command", func() {
	var oldHome string
	var tmpDir string

	BeforeEach(func() {
		oldHome = os.Getenv("HOME")
		tmpDir = GinkgoT().TempDir()
		Expect(os.Setenv("HOME", tmpDir)).To(Succeed())
	})

	AfterEach(func() {
		Expect(os.Setenv("HOME", oldHome)).To(Succeed())
		metricsFlags.days = 7
		metricsFlags.export = ""
		metricsFlags.prometheus = false
		metricsFlags.compact = false
		metricsFlags.analyse = false
		metricsFlags.fleet = false
	})

	It("returns nil when no metrics exist", func() {
		Expect(runMetrics(nil, nil)).To(Succeed())
	})

	It("renders and exports a metrics report", func() {
		metricsPath := filepath.Join(tmpDir, ".cursor", "hooks", "metrics.jsonl")
		Expect(metrics.Record(metricsPath, metrics.Event{
			Timestamp:  time.Now().UTC().Add(-1 * time.Hour),
			Hook:       "track",
			Action:     "record",
			Category:   "skill",
			Detail:     "product-management",
			DurationMs: 42,
		})).To(Succeed())

		metricsFlags.export = filepath.Join(tmpDir, "report.md")
		Expect(runMetrics(nil, nil)).To(Succeed())
		Expect(metricsFlags.export).To(BeAnExistingFile())
	})

	It("supports compact mode", func() {
		metricsPath := filepath.Join(tmpDir, ".cursor", "hooks", "metrics.jsonl")
		Expect(metrics.Record(metricsPath, metrics.Event{
			Timestamp: nowForMetricsTest().Add(-1 * time.Hour),
			Hook:      "guard-shell",
			Action:    "allow",
			Category:  "shell",
			LatencyMs: 1,
		})).To(Succeed())

		metricsFlags.compact = true
		Expect(runMetrics(nil, nil)).To(Succeed())
	})

	It("exports hook hit-rate metrics in Prometheus format", func() {
		metricsPath := filepath.Join(tmpDir, ".cursor", "hooks", "metrics.jsonl")
		Expect(metrics.Record(metricsPath, metrics.Event{
			Timestamp: time.Now().UTC().Add(-1 * time.Hour),
			Category:  "git",
			Action:    "mutation",
			Detail:    "runx git commit",
		})).To(Succeed())
		Expect(metrics.Record(metricsPath, metrics.Event{
			Timestamp: time.Now().UTC().Add(-1 * time.Hour),
			Hook:      "pre-push",
			Action:    "allow",
		})).To(Succeed())

		metricsFlags.prometheus = true
		Expect(runMetrics(nil, nil)).To(Succeed())
	})

	It("renders fleet EvoLoop parity mode", func() {
		fake := &fakeEvoloopClient{capsules: []evoloop.Capsule{
			{Kind: evoloop.KindRollup, Machine: "test-host-1", Day: "2026-04-26", Cycles: 10, Improved: 8, LastKPI: 1.2},
			{Kind: evoloop.KindRollup, Machine: "wsl2", Day: "2026-04-26", Cycles: 4, Improved: 4, LastKPI: 0.6},
		}}
		orig := evoloopFactory
		evoloopFactory = func(_ config.Paths, debug io.Writer) (evoloopClient, error) {
			fake.lastDebug = debug
			return fake, nil
		}
		DeferCleanup(func() { evoloopFactory = orig })

		metricsFlags.fleet = true
		Expect(runMetrics(nil, nil)).To(Succeed())
		Expect(fake.calls).To(Equal(1))
		Expect(fake.lastOpts.Kinds).To(Equal([]evoloop.CapsuleKind{evoloop.KindRollup}))
	})
})

func nowForMetricsTest() time.Time {
	return time.Now().UTC()
}
