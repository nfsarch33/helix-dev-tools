package cli

import (
	"fmt"
	"io"
	"runtime/debug"
)

var readBuildInfo = debug.ReadBuildInfo

func effectiveVersion(version string) string {
	if version == "" {
		version = "dev"
	}
	if info, ok := readBuildInfo(); ok && info != nil {
		commit := buildInfoSetting(info, "vcs.revision")
		buildDate := buildInfoSetting(info, "vcs.time")
		if commit != "" || buildDate != "" {
			return fmt.Sprintf("%s (commit %s, built %s)", version, defaultUnknown(commit), defaultUnknown(buildDate))
		}
	}
	return version
}

func printVersion(out io.Writer, version string) {
	fmt.Fprintln(out, "cursor-tools", effectiveVersion(version))
}

func buildInfoSetting(info *debug.BuildInfo, key string) string {
	for _, setting := range info.Settings {
		if setting.Key == key {
			return setting.Value
		}
	}
	return ""
}

func defaultUnknown(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}
