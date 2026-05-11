package sprintgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestScaffold_GoldenV338QA pins the v338 QA scaffold output
// byte-for-byte against the checked-in fixture. Drift in the
// Universal Story Scaffold rows is a regression -- the v337-5
// `cursor-tools sprint-scaffold` CLI is the single source of truth
// for the sprint shape per the new sprint-scaffold-7-stories.mdc rule.
func TestScaffold_GoldenV338QA(t *testing.T) {
	fixture := filepath.Join("testdata", "sprint-v338-qa.expected.md")
	want, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	got := Scaffold("v338", "QA", "Mem0 Tokyo cutover + supervisor soak")
	if got != string(want) {
		t.Errorf("scaffold drift -- diff first %d chars:", 200)
		// Print a tight diff window for quick eyeballing.
		min := len(got)
		if len(want) < min {
			min = len(want)
		}
		for i := 0; i < min; i++ {
			if got[i] != want[i] {
				start := i - 40
				if start < 0 {
					start = 0
				}
				end := i + 40
				if end > min {
					end = min
				}
				t.Errorf("first diff at byte %d", i)
				t.Errorf("got : %q", got[start:end])
				t.Errorf("want: %q", string(want)[start:end])
				return
			}
		}
		if len(got) != len(want) {
			t.Errorf("length differs: got=%d want=%d", len(got), len(want))
		}
	}
}

// TestScaffold_UniversalStoriesPresent verifies that the universal
// stories (KPI + capsule/retro) appear in every generated scaffold
// regardless of theme. The hard rule is documented in
// sprint-scaffold-7-stories.mdc.
func TestScaffold_UniversalStoriesPresent(t *testing.T) {
	out := Scaffold("v339", "MVP", "Concurrency Audit")
	if !strings.Contains(out, "Hygiene KPI") {
		t.Error("missing universal Hygiene KPI story")
	}
	if !strings.Contains(out, "EvoLoop capsule") {
		t.Error("missing universal EvoLoop capsule story")
	}
	if !strings.Contains(out, "Universal Story Scaffold") {
		t.Error("missing scaffold preamble header")
	}
	// All 7 story IDs present.
	for i := 1; i <= 7; i++ {
		needle := "v339-" + intStr(i)
		if !strings.Contains(out, needle) {
			t.Errorf("missing story id %s", needle)
		}
	}
}

func intStr(i int) string {
	return string(rune('0' + i))
}
