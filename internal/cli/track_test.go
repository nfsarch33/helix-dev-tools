package cli

import (
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/nfsarch33/cursor-tools/internal/metrics"
)

var _ = Describe("Track Command", func() {
	Describe("ValidCategories", func() {
		It("includes all expected categories", func() {
			expected := []string{"mcp", "shell", "skill", "subagent", "script", "tool", "check", "custom"}
			Expect(ValidCategories).To(Equal(expected))
		})

		It("validates known categories", func() {
			for _, cat := range ValidCategories {
				Expect(isValidCategory(cat)).To(BeTrue(), "expected %q to be valid", cat)
			}
		})

		It("rejects unknown categories", func() {
			Expect(isValidCategory("bogus")).To(BeFalse())
			Expect(isValidCategory("")).To(BeFalse())
		})
	})
})

var _ = Describe("CategoryStats in Summarise", func() {
	now := time.Now().UTC()
	hour := time.Hour

	var tmpDir string
	var metricsPath string

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "track-test-*")
		Expect(err).NotTo(HaveOccurred())
		metricsPath = filepath.Join(tmpDir, "metrics.jsonl")
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	It("computes per-category stats for tracked events", func() {
		events := []metrics.Event{
			{Timestamp: now.Add(-1 * hour), Hook: "track", Action: "record", Category: "mcp", Detail: "context7.resolve", DurationMs: 100},
			{Timestamp: now.Add(-1 * hour), Hook: "track", Action: "record", Category: "mcp", Detail: "context7.resolve", DurationMs: 200},
			{Timestamp: now.Add(-1 * hour), Hook: "track", Action: "record", Category: "mcp", Detail: "fetch.get", DurationMs: 50},
			{Timestamp: now.Add(-1 * hour), Hook: "track", Action: "record", Category: "skill", Detail: "ui-ux-pro-max", DurationMs: 1500},
			{Timestamp: now.Add(-1 * hour), Hook: "track", Action: "record", Category: "skill", Detail: "skill-creator", DurationMs: 800},
			{Timestamp: now.Add(-1 * hour), Hook: "track", Action: "record", Category: "shell", Detail: "go test", DurationMs: 3000},
		}

		summary := metrics.Summarise(events, now.Add(-24*hour))
		Expect(summary.Categories).To(HaveLen(3))

		catMap := make(map[string]metrics.CategoryStats)
		for _, c := range summary.Categories {
			catMap[c.Category] = c
		}

		mcp := catMap["mcp"]
		Expect(mcp.Count).To(Equal(3))
		Expect(mcp.MinDuration).To(Equal(int64(50)))
		Expect(mcp.MaxDuration).To(Equal(int64(200)))

		skill := catMap["skill"]
		Expect(skill.Count).To(Equal(2))
		Expect(skill.AvgDuration).To(BeNumerically("~", 1150, 1))

		shell := catMap["shell"]
		Expect(shell.Count).To(Equal(1))
		Expect(shell.MaxDuration).To(Equal(int64(3000)))
	})

	It("excludes events without category", func() {
		events := []metrics.Event{
			{Timestamp: now.Add(-1 * hour), Hook: "guard-shell", Action: "allow", LatencyMs: 2, Detail: "ls"},
			{Timestamp: now.Add(-1 * hour), Hook: "track", Action: "record", Category: "mcp", Detail: "test", DurationMs: 100},
		}

		summary := metrics.Summarise(events, now.Add(-24*hour))
		Expect(summary.Categories).To(HaveLen(1))
		Expect(summary.Categories[0].Category).To(Equal("mcp"))
	})

	It("counts zero-duration events (hook events with 0ms latency)", func() {
		events := []metrics.Event{
			{Timestamp: now.Add(-1 * hour), Hook: "guard-shell", Action: "allow", Category: "shell", LatencyMs: 0},
			{Timestamp: now.Add(-1 * hour), Hook: "track", Action: "record", Category: "shell", DurationMs: 100},
		}

		summary := metrics.Summarise(events, now.Add(-24*hour))
		Expect(summary.Categories).To(HaveLen(1))
		Expect(summary.Categories[0].Count).To(Equal(2))
	})

	It("returns empty categories for no tracked events", func() {
		events := []metrics.Event{
			{Timestamp: now.Add(-1 * hour), Hook: "guard-shell", Action: "allow", LatencyMs: 2},
		}

		summary := metrics.Summarise(events, now.Add(-24*hour))
		Expect(summary.Categories).To(BeEmpty())
	})

	It("computes P95 correctly for many events", func() {
		var events []metrics.Event
		for i := 1; i <= 100; i++ {
			events = append(events, metrics.Event{
				Timestamp:  now.Add(-1 * hour),
				Hook:       "track",
				Action:     "record",
				Category:   "mcp",
				Detail:     "perf-test",
				DurationMs: int64(i * 10),
			})
		}

		summary := metrics.Summarise(events, now.Add(-24*hour))
		Expect(summary.Categories).To(HaveLen(1))
		Expect(summary.Categories[0].P95Duration).To(Equal(int64(960)))
	})

	It("records manual events to JSONL", func() {
		err := metrics.Record(metricsPath, metrics.Event{
			Hook:       "track",
			Action:     "record",
			Category:   "skill",
			Detail:     "test-skill",
			DurationMs: 500,
		})
		Expect(err).NotTo(HaveOccurred())

		events, err := metrics.Load(metricsPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(events).To(HaveLen(1))
		Expect(events[0].Category).To(Equal("skill"))
		Expect(events[0].DurationMs).To(Equal(int64(500)))
	})

	It("includes category in Markdown report", func() {
		events := []metrics.Event{
			{Timestamp: now.Add(-1 * hour), Hook: "track", Action: "record", Category: "mcp", Detail: "test", DurationMs: 100},
		}
		summary := metrics.Summarise(events, now.Add(-24*hour))
		md := summary.Markdown()
		Expect(md).To(ContainSubstring("Operation Timing by Category"))
		Expect(md).To(ContainSubstring("mcp"))
	})

	It("includes category in Compact output", func() {
		events := []metrics.Event{
			{Timestamp: now.Add(-1 * hour), Hook: "track", Action: "record", Category: "mcp", Detail: "test", DurationMs: 100},
			{Timestamp: now.Add(-1 * hour), Hook: "guard-shell", Action: "allow", LatencyMs: 2},
		}
		summary := metrics.Summarise(events, now.Add(-24*hour))
		compact := summary.Compact(7)
		Expect(compact).To(ContainSubstring("mcp=1@100ms"))
	})

	It("preserves backward compatibility for ExitCode field", func() {
		err := metrics.Record(metricsPath, metrics.Event{
			Hook:       "track",
			Action:     "record",
			Category:   "shell",
			Detail:     "failing-cmd",
			DurationMs: 200,
			ExitCode:   1,
		})
		Expect(err).NotTo(HaveOccurred())

		events, err := metrics.Load(metricsPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(events[0].ExitCode).To(Equal(1))
	})

	It("sorts categories by count descending", func() {
		events := []metrics.Event{
			{Timestamp: now.Add(-1 * hour), Hook: "track", Action: "record", Category: "shell", Detail: "a", DurationMs: 10},
			{Timestamp: now.Add(-1 * hour), Hook: "track", Action: "record", Category: "mcp", Detail: "b", DurationMs: 20},
			{Timestamp: now.Add(-1 * hour), Hook: "track", Action: "record", Category: "mcp", Detail: "c", DurationMs: 30},
			{Timestamp: now.Add(-1 * hour), Hook: "track", Action: "record", Category: "mcp", Detail: "d", DurationMs: 40},
		}
		summary := metrics.Summarise(events, now.Add(-24*hour))
		Expect(summary.Categories[0].Category).To(Equal("mcp"))
		Expect(summary.Categories[1].Category).To(Equal("shell"))
	})
})

var _ = Describe("Track execution paths", func() {
	var oldHome string
	var tmpDir string

	BeforeEach(func() {
		oldHome = os.Getenv("HOME")
		tmpDir = GinkgoT().TempDir()
		Expect(os.Setenv("HOME", tmpDir)).To(Succeed())
	})

	AfterEach(func() {
		Expect(os.Setenv("HOME", oldHome)).To(Succeed())
		trackFlags.category = "custom"
		trackFlags.name = ""
		trackFlags.durationMs = 0
	})

	It("records manual track events", func() {
		trackFlags.category = "skill"
		trackFlags.name = "manual-skill"
		trackFlags.durationMs = 123

		Expect(runTrack(nil, nil)).To(Succeed())

		events, err := metrics.Load(filepath.Join(tmpDir, ".cursor", "hooks", "metrics.jsonl"))
		Expect(err).NotTo(HaveOccurred())
		Expect(events).To(HaveLen(1))
		Expect(events[0].Category).To(Equal("skill"))
		Expect(events[0].Detail).To(Equal("manual-skill"))
		Expect(events[0].DurationMs).To(Equal(int64(123)))
	})

	It("records wrapper executions", func() {
		trackFlags.category = "tool"
		trackFlags.name = "wrapper"

		Expect(runTrack(nil, []string{"sh", "-c", "exit 0"})).To(Succeed())

		events, err := metrics.Load(filepath.Join(tmpDir, ".cursor", "hooks", "metrics.jsonl"))
		Expect(err).NotTo(HaveOccurred())
		Expect(events).To(HaveLen(1))
		Expect(events[0].Category).To(Equal("tool"))
		Expect(events[0].Detail).To(Equal("wrapper"))
		Expect(events[0].ExitCode).To(Equal(0))
	})

	It("rejects invalid categories", func() {
		trackFlags.category = "bogus"
		trackFlags.name = "bad"
		err := runTrack(nil, nil)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("invalid category"))
	})
})
