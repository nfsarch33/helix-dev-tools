package clilog_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/nfsarch33/helix-dev-tools/internal/clilog"
)

func TestClilog(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Clilog Suite")
}

var _ = Describe("Clilog", func() {
	BeforeEach(func() {
		clilog.SetColor(false)
	})

	Describe("Prefixed", func() {
		It("tracks error count", func() {
			p := clilog.NewPrefixed("[test]")
			Expect(p.Errors()).To(Equal(0))
			p.Error("something broke")
			Expect(p.Errors()).To(Equal(1))
			p.Error("another failure")
			Expect(p.Errors()).To(Equal(2))
		})

		It("does not increment errors on Info or Warn", func() {
			p := clilog.NewPrefixed("[test]")
			p.Info("informational")
			p.Warn("a warning")
			Expect(p.Errors()).To(Equal(0))
		})
	})

	Describe("SetColor", func() {
		It("disables colour without panic", func() {
			Expect(func() { clilog.SetColor(false) }).NotTo(Panic())
		})

		It("enables colour without panic", func() {
			Expect(func() { clilog.SetColor(true) }).NotTo(Panic())
		})
	})

	Describe("output functions", func() {
		It("does not panic on Info", func() {
			Expect(func() { clilog.Info("test %s", "msg") }).NotTo(Panic())
		})

		It("does not panic on Success", func() {
			Expect(func() { clilog.Success("test %s", "msg") }).NotTo(Panic())
		})

		It("does not panic on Warn", func() {
			Expect(func() { clilog.Warn("test %s", "msg") }).NotTo(Panic())
		})

		It("does not panic on Error", func() {
			Expect(func() { clilog.Error("test %s", "msg") }).NotTo(Panic())
		})

		It("does not panic on Header", func() {
			Expect(func() { clilog.Header("Test Header") }).NotTo(Panic())
		})

		It("does not panic on Divider", func() {
			Expect(func() { clilog.Divider() }).NotTo(Panic())
		})

		It("does not panic on Pass", func() {
			Expect(func() { clilog.Pass("test %s", "pass") }).NotTo(Panic())
		})

		It("does not panic on Fail", func() {
			Expect(func() { clilog.Fail("test %s", "fail") }).NotTo(Panic())
		})

		It("does not panic on Result", func() {
			Expect(func() { clilog.Result("suite", 5, 10) }).NotTo(Panic())
		})

		It("does not panic on Summary", func() {
			Expect(func() { clilog.Summary(10, 10) }).NotTo(Panic())
		})
	})
})
