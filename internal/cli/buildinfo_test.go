package cli

import (
	"bytes"
	"runtime/debug"
	"strings"
	"testing"
)

func TestEffectiveVersionFallsBackToBuildInfo(t *testing.T) {
	original := readBuildInfo
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "1365d92deadbeef"},
				{Key: "vcs.time", Value: "2026-05-14T11:30:00Z"},
			},
		}, true
	}
	defer func() { readBuildInfo = original }()

	got := effectiveVersion("dev")
	if !strings.Contains(got, "1365d92deadbeef") {
		t.Fatalf("effectiveVersion() = %q", got)
	}
	if !strings.Contains(got, "2026-05-14T11:30:00Z") {
		t.Fatalf("effectiveVersion() = %q", got)
	}
}

func TestPrintVersionUsesCommandWriter(t *testing.T) {
	original := readBuildInfo
	readBuildInfo = func() (*debug.BuildInfo, bool) { return nil, false }
	defer func() { readBuildInfo = original }()

	var out bytes.Buffer
	printVersion(&out, "v1.2.3")
	if got := out.String(); got != "cursor-tools v1.2.3\n" {
		t.Fatalf("printVersion() = %q", got)
	}
}
