package logger_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/nfsarch33/cursor-tools/internal/logger"
)

func TestLogger(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Logger Suite")
}

var _ = Describe("Logger", func() {
	var tmpDir string

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "logger-test-*")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	Describe("Log", func() {
		It("creates file and writes timestamped message", func() {
			logPath := filepath.Join(tmpDir, "test.log")
			l := logger.New(logPath)
			l.Log("hello world")

			data, err := os.ReadFile(logPath)
			Expect(err).NotTo(HaveOccurred())
			content := string(data)
			Expect(content).To(ContainSubstring("hello world"))
			Expect(content).To(MatchRegexp(`\[\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z\]`))
		})

		It("appends multiple messages", func() {
			logPath := filepath.Join(tmpDir, "test.log")
			l := logger.New(logPath)
			l.Log("first")
			l.Log("second")

			data, err := os.ReadFile(logPath)
			Expect(err).NotTo(HaveOccurred())
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			Expect(lines).To(HaveLen(2))
		})
	})

	Describe("Rotate", func() {
		It("rotates when file exceeds max bytes", func() {
			logPath := filepath.Join(tmpDir, "test.log")
			l := logger.New(logPath).WithMaxBytes(50)

			l.Log("this is a message that will exceed the fifty byte limit easily")
			l.Rotate()

			Expect(filepath.Join(tmpDir, "test.log.1")).To(BeAnExistingFile())
			data, _ := os.ReadFile(logPath)
			Expect(string(data)).To(ContainSubstring("log rotated"))
		})

		It("does not rotate when file is small", func() {
			logPath := filepath.Join(tmpDir, "test.log")
			l := logger.New(logPath).WithMaxBytes(10000)

			l.Log("small")
			l.Rotate()

			Expect(filepath.Join(tmpDir, "test.log.1")).NotTo(BeAnExistingFile())
		})
	})
})
