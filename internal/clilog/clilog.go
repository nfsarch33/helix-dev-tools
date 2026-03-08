package clilog

import (
	"fmt"
	"os"
	"strings"
)

var colorEnabled = detectTerminal()

func detectTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// SetColor overrides auto-detection (useful for tests).
func SetColor(enabled bool) { colorEnabled = enabled }

const (
	reset  = "\033[0m"
	red    = "\033[31m"
	green  = "\033[32m"
	yellow = "\033[33m"
	cyan   = "\033[36m"
	bold   = "\033[1m"
	dim    = "\033[2m"
)

func paint(c, s string) string {
	if !colorEnabled {
		return s
	}
	return c + s + reset
}

func Info(format string, args ...interface{}) {
	fmt.Fprintf(os.Stdout, "  %s  %s\n", paint(cyan, "INFO"), fmt.Sprintf(format, args...))
}

func Success(format string, args ...interface{}) {
	fmt.Fprintf(os.Stdout, "  %s    %s\n", paint(green, "OK"), fmt.Sprintf(format, args...))
}

func Warn(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "  %s  %s\n", paint(yellow, "WARN"), fmt.Sprintf(format, args...))
}

func Error(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "  %s %s\n", paint(red, "ERROR"), fmt.Sprintf(format, args...))
}

func Header(title string) {
	line := strings.Repeat("=", 60)
	fmt.Println(paint(bold, line))
	fmt.Println(paint(bold, "  "+title))
	fmt.Println(paint(bold, line))
}

func Divider() {
	fmt.Println(strings.Repeat("-", 62))
}

func Pass(format string, args ...interface{}) {
	fmt.Printf("    %s  %s\n", paint(green, "PASS"), fmt.Sprintf(format, args...))
}

func Fail(format string, args ...interface{}) {
	fmt.Printf("    %s  %s\n", paint(red, "FAIL"), fmt.Sprintf(format, args...))
}

func Result(label string, pass, total int) {
	pct := float64(pass) / float64(total) * 100
	status := paint(green, "PASS")
	if pass < total {
		status = paint(red, "FAIL")
	}
	fmt.Printf("  %s  %d/%d (%.0f%%)  %s\n", status, pass, total, pct, label)
}

func Summary(pass, total int) {
	Divider()
	pct := float64(pass) / float64(total) * 100
	fmt.Printf("\n  %d/%d assertions passed (%.0f%%)\n\n", pass, total, pct)
	if pass == total {
		fmt.Println("  " + paint(green+bold, "ALL TESTS PASSED.") + " System is healthy.")
	} else {
		fmt.Printf("  %s detected.\n", paint(red+bold, fmt.Sprintf("%d FAILURES", total-pass)))
	}
	fmt.Println()
}

// Prefixed wraps CLI output with a consistent prefix and tracks errors.
type Prefixed struct {
	prefix string
	errors int
}

func NewPrefixed(prefix string) *Prefixed {
	return &Prefixed{prefix: prefix}
}

func (p *Prefixed) Info(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stdout, "%s %s\n", paint(cyan, p.prefix), msg)
}

func (p *Prefixed) Warn(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s %s %s\n", paint(cyan, p.prefix), paint(yellow, "WARN:"), msg)
}

func (p *Prefixed) Error(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s %s %s\n", paint(cyan, p.prefix), paint(red, "ERROR:"), msg)
	p.errors++
}

func (p *Prefixed) Errors() int { return p.errors }
