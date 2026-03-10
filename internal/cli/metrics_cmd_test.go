package cli

import (
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/nfsarch33/cursor-tools/internal/metrics"
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
		metricsFlags.compact = false
		metricsFlags.analyse = false
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
})

func nowForMetricsTest() time.Time {
	return time.Now().UTC()
}
