package logger_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/nfsarch33/helix-dev-tools/internal/logger"
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

	Describe("New", func() {
		It("creates a logger with the given path", func() {
			logPath := filepath.Join(tmpDir, "test.log")
			l := logger.New(logPath)
			Expect(l).NotTo(BeNil())
		})
	})

	Describe("WithMaxBytes", func() {
		It("returns the same logger for chaining", func() {
			logPath := filepath.Join(tmpDir, "test.log")
			l := logger.New(logPath)
			l2 := l.WithMaxBytes(100)
			Expect(l2).To(BeIdenticalTo(l))
		})
	})

	Describe("Log", func() {
		It("creates file and writes timestamped message", func() {
			logPath := filepath.Join(tmpDir, "test.log")
			l := logger.New(logPath)
			l.Log("hello world")

			data, err := os.ReadFile(logPath)
			Expect(err).NotTo(HaveOccurred())
			var entry map[string]any
			Expect(json.Unmarshal(data, &entry)).To(Succeed())
			Expect(entry["msg"]).To(Equal("hello world"))
			Expect(entry["ts"]).NotTo(BeEmpty())
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

		It("creates nested directories if needed", func() {
			logPath := filepath.Join(tmpDir, "nested", "deep", "test.log")
			l := logger.New(logPath)
			l.Log("nested message")

			data, err := os.ReadFile(logPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(ContainSubstring("nested message"))
		})

		It("handles concurrent writes safely", func() {
			logPath := filepath.Join(tmpDir, "concurrent.log")
			l := logger.New(logPath)

			done := make(chan struct{}, 10)
			for i := 0; i < 10; i++ {
				go func(n int) {
					l.Log("concurrent message")
					done <- struct{}{}
				}(i)
			}
			for i := 0; i < 10; i++ {
				<-done
			}

			data, err := os.ReadFile(logPath)
			Expect(err).NotTo(HaveOccurred())
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			Expect(lines).To(HaveLen(10))
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
			Expect(strings.TrimSpace(string(data))).To(BeEmpty())
		})

		It("does not rotate when file is small", func() {
			logPath := filepath.Join(tmpDir, "test.log")
			l := logger.New(logPath).WithMaxBytes(10000)

			l.Log("small")
			l.Rotate()

			Expect(filepath.Join(tmpDir, "test.log.1")).NotTo(BeAnExistingFile())
		})

		It("does nothing when file does not exist", func() {
			logPath := filepath.Join(tmpDir, "nonexistent.log")
			l := logger.New(logPath).WithMaxBytes(50)
			l.Rotate()
			Expect(logPath).NotTo(BeAnExistingFile())
		})
	})

	Describe("RotateAll", func() {
		It("rotates files exceeding default threshold", func() {
			for _, name := range []string{"a.log", "b.log"} {
				p := filepath.Join(tmpDir, name)
				Expect(os.WriteFile(p, make([]byte, 600000), 0o644)).To(Succeed())
			}

			logger.RotateAll(tmpDir, []string{"a.log", "b.log"})

			Expect(filepath.Join(tmpDir, "a.log.1")).To(BeAnExistingFile())
			Expect(filepath.Join(tmpDir, "b.log.1")).To(BeAnExistingFile())
		})

		It("skips files that are under threshold", func() {
			p := filepath.Join(tmpDir, "small.log")
			l := logger.New(p)
			l.Log("tiny")

			logger.RotateAll(tmpDir, []string{"small.log"})
			Expect(filepath.Join(tmpDir, "small.log.1")).NotTo(BeAnExistingFile())
		})

		It("handles empty names list", func() {
			Expect(func() { logger.RotateAll(tmpDir, nil) }).NotTo(Panic())
			Expect(func() { logger.RotateAll(tmpDir, []string{}) }).NotTo(Panic())
		})

		It("handles non-existent files gracefully", func() {
			Expect(func() { logger.RotateAll(tmpDir, []string{"missing.log"}) }).NotTo(Panic())
		})
	})
})
