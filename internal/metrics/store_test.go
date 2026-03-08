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

	Describe("effectiveDuration", func() {
		It("prefers DurationMs when set", func() {
			e := metrics.Event{DurationMs: 500, LatencyMs: 2}
			Expect(metrics.EffectiveDuration(e)).To(Equal(int64(500)))
		})

		It("falls back to LatencyMs when DurationMs is zero", func() {
			e := metrics.Event{DurationMs: 0, LatencyMs: 3}
			Expect(metrics.EffectiveDuration(e)).To(Equal(int64(3)))
		})

		It("returns zero when both are zero", func() {
			e := metrics.Event{}
			Expect(metrics.EffectiveDuration(e)).To(Equal(int64(0)))
		})
	})

	Describe("buildCategoryStats with hook events", func() {
		now := time.Now().UTC()

		It("counts hook events with Category set", func() {
			events := []metrics.Event{
				{Timestamp: now.Add(-1 * time.Hour), Hook: "guard-shell", Action: "allow", Category: "shell", LatencyMs: 1},
				{Timestamp: now.Add(-1 * time.Hour), Hook: "guard-shell", Action: "allow", Category: "shell", LatencyMs: 0},
				{Timestamp: now.Add(-1 * time.Hour), Hook: "guard-mcp", Action: "allow", Category: "mcp", LatencyMs: 1},
			}
			summary := metrics.Summarise(events, now.Add(-24*time.Hour))
			Expect(summary.Categories).To(HaveLen(2))
		})

		It("uses LatencyMs for hook events without DurationMs", func() {
			events := []metrics.Event{
				{Timestamp: now.Add(-1 * time.Hour), Hook: "guard-shell", Action: "allow", Category: "shell", LatencyMs: 5},
			}
			summary := metrics.Summarise(events, now.Add(-24*time.Hour))
			Expect(summary.Categories).To(HaveLen(1))
			Expect(summary.Categories[0].AvgDuration).To(BeNumerically("==", 5))
		})

		It("mixed tracked and hook events both appear in categories", func() {
			events := []metrics.Event{
				{Timestamp: now.Add(-1 * time.Hour), Hook: "guard-shell", Action: "allow", Category: "shell", LatencyMs: 1},
				{Timestamp: now.Add(-1 * time.Hour), Hook: "track", Action: "record", Category: "shell", DurationMs: 2000},
			}
			summary := metrics.Summarise(events, now.Add(-24*time.Hour))
			Expect(summary.Categories).To(HaveLen(1))
			Expect(summary.Categories[0].Count).To(Equal(2))
		})
	})

	Describe("Summary.Analyse", func() {
		now := time.Now().UTC()

		It("flags high intervention rate", func() {
			events := []metrics.Event{
				{Timestamp: now.Add(-1 * time.Hour), Hook: "guard-shell", Action: "deny", Category: "shell", LatencyMs: 1, Detail: "rm -rf /"},
				{Timestamp: now.Add(-1 * time.Hour), Hook: "guard-shell", Action: "deny", Category: "shell", LatencyMs: 1, Detail: "rm -rf /tmp"},
				{Timestamp: now.Add(-1 * time.Hour), Hook: "guard-shell", Action: "allow", Category: "shell", LatencyMs: 0},
			}
			summary := metrics.Summarise(events, now.Add(-24*time.Hour))
			recs := summary.Analyse()
			found := false
			for _, r := range recs {
				if r.Severity == "warn" && r.Category == "intervention" {
					found = true
				}
			}
			Expect(found).To(BeTrue(), "expected intervention rate recommendation")
		})

		It("flags slow hooks", func() {
			events := []metrics.Event{
				{Timestamp: now.Add(-1 * time.Hour), Hook: "post-edit", Action: "format", Category: "tool", LatencyMs: 200},
			}
			summary := metrics.Summarise(events, now.Add(-24*time.Hour))
			recs := summary.Analyse()
			found := false
			for _, r := range recs {
				if r.Severity == "warn" && r.Category == "latency" {
					found = true
				}
			}
			Expect(found).To(BeTrue(), "expected slow hook recommendation")
		})

		It("flags degrading trend", func() {
			events := []metrics.Event{
				{Timestamp: now.Add(-48 * time.Hour), Hook: "guard-shell", Action: "allow", Category: "shell", LatencyMs: 1},
				{Timestamp: now.Add(-1 * time.Hour), Hook: "guard-shell", Action: "deny", Category: "shell", LatencyMs: 100, Detail: "rm -rf /"},
			}
			summary := metrics.Summarise(events, now.Add(-24*time.Hour))
			recs := summary.Analyse()
			Expect(recs).NotTo(BeEmpty())
		})

		It("returns empty for healthy system", func() {
			events := []metrics.Event{
				{Timestamp: now.Add(-1 * time.Hour), Hook: "guard-shell", Action: "allow", Category: "shell", LatencyMs: 0},
				{Timestamp: now.Add(-1 * time.Hour), Hook: "sanitize-read", Action: "allow", Category: "tool", LatencyMs: 0},
			}
			summary := metrics.Summarise(events, now.Add(-24*time.Hour))
			recs := summary.Analyse()
			Expect(recs).To(BeEmpty())
		})

		It("flags high P95 in tracked category", func() {
			var events []metrics.Event
			for i := 0; i < 20; i++ {
				events = append(events, metrics.Event{
					Timestamp:  now.Add(-1 * time.Hour),
					Hook:       "track",
					Action:     "record",
					Category:   "mcp",
					DurationMs: int64(2000 + i*100),
				})
			}
			summary := metrics.Summarise(events, now.Add(-24*time.Hour))
			Expect(summary.Categories).NotTo(BeEmpty())
			Expect(summary.Categories[0].P95Duration).To(BeNumerically(">", 2000))
			recs := summary.Analyse()
			found := false
			for _, r := range recs {
				if r.Category == "performance" {
					found = true
				}
			}
			Expect(found).To(BeTrue(), "expected P95 performance recommendation")
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
