package semblediscipline

import "testing"

func TestClassify_NotApplicable(t *testing.T) {
	got := Classify("go test ./...")
	if got.Verdict != VerdictNotApplicable {
		t.Fatalf("got %+v, want not_applicable", got)
	}
}

func TestClassify_ExploratoryRg(t *testing.T) {
	got := Classify(`rg "ErrNotFound" ~/cursor-tools`)
	if got.Verdict != VerdictExploratory || got.Tool != "rg" {
		t.Fatalf("got %+v, want exploratory rg", got)
	}
}

func TestClassify_LiteralRgF(t *testing.T) {
	got := Classify(`rg -F 'ErrNotFound' internal/cli`)
	if got.Verdict != VerdictLiteralOK {
		t.Fatalf("got %+v, want literal_ok", got)
	}
}

func TestClassify_ExploratoryGrepRecursive(t *testing.T) {
	got := Classify("grep -r foo .")
	if got.Verdict != VerdictExploratory {
		t.Fatalf("got %+v, want exploratory", got)
	}
}

func TestClassify_LiteralGrepF(t *testing.T) {
	got := Classify("grep -F exact-token path.go")
	if got.Verdict != VerdictLiteralOK {
		t.Fatalf("got %+v, want literal_ok", got)
	}
}

func TestClassify_ExactFileTarget(t *testing.T) {
	got := Classify("grep pattern internal/cli/root.go")
	if got.Verdict != VerdictLiteralOK {
		t.Fatalf("got %+v, want literal_ok for single file", got)
	}
}

func TestClassify_ExploratoryFind(t *testing.T) {
	got := Classify("find . -name '*.go'")
	if got.Verdict != VerdictExploratory || got.Tool != "find" {
		t.Fatalf("got %+v, want exploratory find", got)
	}
}

func TestClassify_CompoundPipeline(t *testing.T) {
	got := Classify("cd /tmp && rg foo")
	if got.Verdict != VerdictExploratory || got.Tool != "rg" {
		t.Fatalf("got %+v, want exploratory rg in pipeline", got)
	}
}
