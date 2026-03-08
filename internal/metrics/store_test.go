package metrics_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/nfsarch33/cursor-tools/internal/metrics"
)

func TestMetrics(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Metrics Suite")
}

var _ = Describe("Metrics Store", func() {
	var tmpDir string
	var metricsPath string

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "metrics-test-*")
		Expect(err).NotTo(HaveOccurred())
		metricsPath = filepath.Join(tmpDir, "metrics.jsonl")
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	Describe("Record", func() {
		It("creates file and writes event", func() {
			err := metrics.Record(metricsPath, metrics.Event{
				Hook:      "guard-shell",
				Action:    "allow",
				LatencyMs: 2,
				Detail:    "ls -la",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(metricsPath).To(BeAnExistingFile())
		})

		It("appends multiple events", func() {
			for i := 0; i < 5; i++ {
				err := metrics.Record(metricsPath, metrics.Event{
					Hook:      "guard-shell",
					Action:    "allow",
					LatencyMs: int64(i),
				})
				Expect(err).NotTo(HaveOccurred())
			}
			events, err := metrics.Load(metricsPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(events).To(HaveLen(5))
		})

		It("auto-fills timestamp when zero", func() {
			err := metrics.Record(metricsPath, metrics.Event{
				Hook:   "test",
				Action: "allow",
			})
			Expect(err).NotTo(HaveOccurred())
			events, _ := metrics.Load(metricsPath)
			Expect(events[0].Timestamp).NotTo(BeZero())
		})

		It("creates nested directories", func() {
			nested := filepath.Join(tmpDir, "a", "b", "metrics.jsonl")
			err := metrics.Record(nested, metrics.Event{Hook: "test", Action: "allow"})
			Expect(err).NotTo(HaveOccurred())
			Expect(nested).To(BeAnExistingFile())
		})
	})

	Describe("Load", func() {
		It("returns nil for missing file", func() {
			events, err := metrics.Load(filepath.Join(tmpDir, "nonexistent.jsonl"))
			Expect(err).NotTo(HaveOccurred())
			Expect(events).To(BeNil())
		})

		It("skips malformed lines", func() {
			err := os.WriteFile(metricsPath, []byte("{\"hook\":\"ok\",\"action\":\"allow\",\"ts\":\"2026-01-01T00:00:00Z\",\"latency_ms\":1}\nnot json\n{\"hook\":\"ok2\",\"action\":\"deny\",\"ts\":\"2026-01-01T00:00:00Z\",\"latency_ms\":2}\n"), 0o644)
			Expect(err).NotTo(HaveOccurred())
			events, err := metrics.Load(metricsPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(events).To(HaveLen(2))
		})
	})

	Describe("Summarise", func() {
		var events []metrics.Event
		now := time.Now().UTC()

		BeforeEach(func() {
			events = []metrics.Event{
				{Timestamp: now.Add(-1 * time.Hour), Hook: "guard-shell", Action: "allow", LatencyMs: 2, Detail: "ls"},
				{Timestamp: now.Add(-1 * time.Hour), Hook: "guard-shell", Action: "deny", LatencyMs: 1, Detail: "rm -rf /"},
				{Timestamp: now.Add(-1 * time.Hour), Hook: "guard-shell", Action: "deny", LatencyMs: 1, Detail: "rm -rf /"},
				{Timestamp: now.Add(-1 * time.Hour), Hook: "guard-shell", Action: "warn", LatencyMs: 3, Detail: "sudo apt"},
				{Timestamp: now.Add(-1 * time.Hour), Hook: "guard-mcp", Action: "allow", LatencyMs: 1, Detail: "search"},
				{Timestamp: now.Add(-1 * time.Hour), Hook: "sanitize-read", Action: "deny", LatencyMs: 1, Detail: ".env"},
				{Timestamp: now.Add(-48 * time.Hour), Hook: "guard-shell", Action: "allow", LatencyMs: 5, Detail: "old event"},
			}
		})

		It("aggregates events since time", func() {
			summary := metrics.Summarise(events, now.Add(-24*time.Hour))
			Expect(summary.TotalEvents).To(Equal(6))
			Expect(summary.Hooks).To(HaveLen(3))
		})

		It("excludes events before since time", func() {
			summary := metrics.Summarise(events, now.Add(-24*time.Hour))
			Expect(summary.TotalEvents).To(Equal(6))
		})

		It("computes correct deny/warn/allow counts", func() {
			summary := metrics.Summarise(events, now.Add(-24*time.Hour))
			var shell metrics.HookStats
			for _, h := range summary.Hooks {
				if h.Hook == "guard-shell" {
					shell = h
					break
				}
			}
			Expect(shell.DenyCount).To(Equal(2))
			Expect(shell.WarnCount).To(Equal(1))
			Expect(shell.AllowCount).To(Equal(1))
			Expect(shell.Total).To(Equal(4))
		})

		It("computes top denied entries", func() {
			summary := metrics.Summarise(events, now.Add(-24*time.Hour))
			Expect(summary.TopDenied).NotTo(BeEmpty())
			Expect(summary.TopDenied[0].Detail).To(Equal("rm -rf /"))
			Expect(summary.TopDenied[0].Count).To(Equal(2))
		})

		It("returns zero for empty events", func() {
			summary := metrics.Summarise(nil, now.Add(-24*time.Hour))
			Expect(summary.TotalEvents).To(Equal(0))
			Expect(summary.Hooks).To(BeEmpty())
		})
	})

	Describe("Summary.Markdown", func() {
		It("renders valid markdown with hook table", func() {
			events := []metrics.Event{
				{Timestamp: time.Now().UTC(), Hook: "guard-shell", Action: "allow", LatencyMs: 2},
				{Timestamp: time.Now().UTC(), Hook: "guard-shell", Action: "deny", LatencyMs: 1, Detail: "rm -rf /"},
			}
			summary := metrics.Summarise(events, time.Now().UTC().Add(-1*time.Hour))
			md := summary.Markdown()
			Expect(md).To(ContainSubstring("# System Performance Report"))
			Expect(md).To(ContainSubstring("guard-shell"))
			Expect(md).To(ContainSubstring("Intervention Rate"))
		})

		It("handles empty data gracefully", func() {
			summary := metrics.Summarise(nil, time.Now().UTC().Add(-1*time.Hour))
			md := summary.Markdown()
			Expect(md).To(ContainSubstring("No metrics data"))
		})
	})
})
