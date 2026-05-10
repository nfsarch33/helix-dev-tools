package sprintgen

import (
	"strings"
	"testing"
)

func TestScaffold_DefaultStoryCount(t *testing.T) {
	out := Scaffold("v337", "MVP", "Workspace Cleanup + Daemon")
	if !strings.Contains(out, "v337-1") {
		t.Error("output should contain story v337-1")
	}
	if !strings.Contains(out, "v337-6") {
		t.Error("output should contain KPI story v337-6")
	}
	if !strings.Contains(out, "v337-7") {
		t.Error("output should contain capsule story v337-7")
	}
}

func TestScaffold_ContainsHeader(t *testing.T) {
	out := Scaffold("v338", "QA", "Mem0 Cutover")
	if !strings.Contains(out, "## v338 (QA)") {
		t.Error("output should contain sprint header")
	}
	if !strings.Contains(out, "Mem0 Cutover") {
		t.Error("output should contain theme")
	}
}

func TestScaffold_UniversalStories(t *testing.T) {
	out := Scaffold("v339", "MVP", "Concurrency Audit")
	if !strings.Contains(out, "Hygiene KPI") {
		t.Error("output should contain universal KPI story")
	}
	if !strings.Contains(out, "EvoLoop capsule") {
		t.Error("output should contain universal capsule story")
	}
}

func TestScaffold_OSS_Evidence(t *testing.T) {
	out := Scaffold("v340", "QA", "Sentrux Deep Dive")
	if !strings.Contains(out, "OSS evidence") {
		t.Error("output should contain OSS evidence field per R1 gate")
	}
}

func TestScaffold_Empty_Sprint(t *testing.T) {
	out := Scaffold("", "MVP", "test")
	if out != "" {
		t.Error("empty sprint ID should return empty string")
	}
}
