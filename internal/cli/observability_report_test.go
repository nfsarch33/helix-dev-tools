package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestObservabilityReport_ReadsAllStreams(t *testing.T) {
	tmp := t.TempDir()

	writeFixture(t, tmp, "git.ndjson", []string{
		`{"time":"2026-05-14T01:00:00+10:00","level":"INFO","msg":"git.exec","tool":"runx-git","repo":"global-kb","verb":"status","err":""}`,
		`{"time":"2026-05-14T01:30:00+10:00","level":"INFO","msg":"git.exec","tool":"runx-git","repo":"router","verb":"fetch","err":""}`,
		`{"time":"2026-05-14T02:00:00+10:00","level":"INFO","msg":"git.exec","tool":"runx-git","repo":"global-kb","verb":"push","err":""}`,
	})
	writeFixture(t, tmp, "ssh.ndjson", []string{
		`{"time":"2026-05-14T01:15:00+10:00","level":"INFO","msg":"ssh.exec","tool":"runx-ssh","target":"remote-host","err":""}`,
		`{"time":"2026-05-14T02:15:00+10:00","level":"INFO","msg":"ssh.exec","tool":"runx-ssh","target":"gpu-host-1","err":""}`,
	})
	writeFixture(t, tmp, "empty.ndjson", nil)

	var buf bytes.Buffer
	err := runObservabilityReport(&buf, tmp, "7d")
	if err != nil {
		t.Fatalf("runObservabilityReport() error = %v", err)
	}

	out := buf.String()

	if !strings.Contains(out, "git.ndjson") {
		t.Errorf("output should list git.ndjson stream, got:\n%s", out)
	}
	if !strings.Contains(out, "ssh.ndjson") {
		t.Errorf("output should list ssh.ndjson stream, got:\n%s", out)
	}

	if !strings.Contains(out, "3") {
		t.Errorf("output should contain count 3 for git stream, got:\n%s", out)
	}
	if !strings.Contains(out, "2") {
		t.Errorf("output should contain count 2 for ssh stream, got:\n%s", out)
	}
}

func TestObservabilityReport_TimeRangeComputed(t *testing.T) {
	tmp := t.TempDir()

	writeFixture(t, tmp, "tunnel.ndjson", []string{
		`{"time":"2026-05-13T10:00:00+10:00","level":"INFO","msg":"tunnel.start"}`,
		`{"time":"2026-05-14T10:00:00+10:00","level":"INFO","msg":"tunnel.stop"}`,
	})

	var buf bytes.Buffer
	err := runObservabilityReport(&buf, tmp, "7d")
	if err != nil {
		t.Fatalf("runObservabilityReport() error = %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "2026-05-13") {
		t.Errorf("output should contain start date 2026-05-13, got:\n%s", out)
	}
	if !strings.Contains(out, "2026-05-14") {
		t.Errorf("output should contain end date 2026-05-14, got:\n%s", out)
	}
}

func TestObservabilityReport_HourlyTimeline(t *testing.T) {
	tmp := t.TempDir()

	writeFixture(t, tmp, "git.ndjson", []string{
		`{"time":"2026-05-14T01:00:00+10:00","level":"INFO","msg":"a"}`,
		`{"time":"2026-05-14T01:30:00+10:00","level":"INFO","msg":"b"}`,
		`{"time":"2026-05-14T02:00:00+10:00","level":"INFO","msg":"c"}`,
	})
	writeFixture(t, tmp, "ssh.ndjson", []string{
		`{"time":"2026-05-14T01:15:00+10:00","level":"INFO","msg":"d"}`,
	})

	var buf bytes.Buffer
	err := runObservabilityReport(&buf, tmp, "7d")
	if err != nil {
		t.Fatalf("runObservabilityReport() error = %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Hourly") || !strings.Contains(out, "Timeline") {
		t.Errorf("output should contain hourly timeline section, got:\n%s", out)
	}
}

func TestObservabilityReport_EmptyDirectory(t *testing.T) {
	tmp := t.TempDir()

	var buf bytes.Buffer
	err := runObservabilityReport(&buf, tmp, "7d")
	if err != nil {
		t.Fatalf("empty dir should not error: %v", err)
	}
	if !strings.Contains(buf.String(), "no NDJSON") {
		t.Errorf("should report no data, got:\n%s", buf.String())
	}
}

func TestObservabilityReport_MarkdownOutput(t *testing.T) {
	tmp := t.TempDir()

	writeFixture(t, tmp, "git.ndjson", []string{
		`{"time":"2026-05-14T01:00:00+10:00","level":"INFO","msg":"a"}`,
	})

	var buf bytes.Buffer
	err := runObservabilityReport(&buf, tmp, "7d")
	if err != nil {
		t.Fatalf("runObservabilityReport() error = %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "# ") {
		t.Errorf("output should be markdown with headers, got:\n%s", out)
	}
	if !strings.Contains(out, "|") {
		t.Errorf("output should contain markdown tables, got:\n%s", out)
	}
}

func writeFixture(t *testing.T, dir, name string, lines []string) {
	t.Helper()
	content := strings.Join(lines, "\n")
	if len(lines) > 0 {
		content += "\n"
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
