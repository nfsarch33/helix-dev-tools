package sprintgen

// Bench-style helper kept inside _test.go so it never ships in the
// production binary. Used by `go test -run TestScaffold_GoldenV338QA
// -update` (planned) to regenerate the fixture; not actually exposed
// via a flag yet -- v338-5 may surface it.
//
// To regenerate the fixture by hand:
//
//	go test ./internal/sprintgen/ -run TestScaffold_DumpV338QA -v
//	# copy stdout block into internal/sprintgen/testdata/sprint-v338-qa.expected.md

import (
	"fmt"
	"os"
	"testing"
)

func TestScaffold_DumpV338QA(t *testing.T) {
	dest := os.Getenv("SPRINTGEN_DUMP")
	if dest == "" {
		t.Skip("set SPRINTGEN_DUMP=<file> to dump fixture")
	}
	out := Scaffold("v338", "QA", "Mem0 Tokyo cutover + supervisor soak")
	if err := os.WriteFile(dest, []byte(out), 0o644); err != nil {
		t.Fatalf("write %s: %v", dest, err)
	}
	fmt.Fprintf(os.Stderr, "wrote %d bytes to %s\n", len(out), dest)
}
