package cli

import (
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/nfsarch33/cursor-tools/internal/config"
	"github.com/nfsarch33/cursor-tools/internal/metrics"
)

var _ = Describe("doctor command wiring", func() {
	It("registers expected doctor subcommands", func() {
		Expect(doctorCmd.Name()).To(Equal("doctor"))
		names := []string{}
		for _, cmd := range doctorCmd.Commands() {
			names = append(names, cmd.Name())
		}
		Expect(names).To(ContainElements("install", "mcp", "platform", "resume"))
	})

	It("registers doctor on the root command", func() {
		names := []string{}
		for _, cmd := range rootCmd.Commands() {
			names = append(names, cmd.Name())
		}
		Expect(names).To(ContainElement("doctor"))
	})
})

var _ = Describe("check metrics helpers", func() {
	It("records a passing self-check event", func() {
		home := GinkgoT().TempDir()
		oldHome := os.Getenv("HOME")
		Expect(os.Setenv("HOME", home)).To(Succeed())
		defer os.Setenv("HOME", oldHome)

		started := time.Now().Add(-150 * time.Millisecond)
		recordCheckRun("doctor-test", started, 5, 5)

		p := config.DefaultPaths()
		events, err := metrics.Load(p.MetricsFile())
		Expect(err).NotTo(HaveOccurred())
		Expect(events).To(HaveLen(1))
		Expect(events[0].Category).To(Equal("check"))
		Expect(events[0].Action).To(Equal("pass"))
		Expect(events[0].Detail).To(Equal("doctor-test"))
		Expect(events[0].PassedCount).To(Equal(5))
		Expect(events[0].TotalCount).To(Equal(5))
		Expect(events[0].DurationMs).To(BeNumerically(">=", 0))
	})

	It("records a failing self-check event", func() {
		home := GinkgoT().TempDir()
		oldHome := os.Getenv("HOME")
		Expect(os.Setenv("HOME", home)).To(Succeed())
		defer os.Setenv("HOME", oldHome)

		recordCheckRun("health-check", time.Now(), 8, 10)

		p := config.DefaultPaths()
		events, err := metrics.Load(filepath.Join(p.HooksDir, "metrics.jsonl"))
		Expect(err).NotTo(HaveOccurred())
		Expect(events).To(HaveLen(1))
		Expect(events[0].Action).To(Equal("fail"))
		Expect(events[0].PassedCount).To(Equal(8))
		Expect(events[0].TotalCount).To(Equal(10))
	})
})

var _ = Describe("percentage", func() {
	It("returns zero for empty denominator", func() {
		Expect(percentage(1, 0)).To(Equal(0.0))
	})

	It("computes percentages", func() {
		Expect(percentage(3, 4)).To(BeNumerically("==", 75.0))
	})
})
