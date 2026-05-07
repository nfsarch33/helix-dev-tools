package identity

import "testing"

func TestSelfTest_DetectsLeakedZendeskTokenFamily(t *testing.T) {
	result := SelfTest(map[string]string{
		"GITHUB_TOKEN": "set",
	})
	if result.Pass {
		t.Fatal("SelfTest must fail when a token variable is set")
	}
	if len(result.Failures) != 1 {
		t.Fatalf("failures = %#v, want exactly one", result.Failures)
	}
	if result.Failures[0] != "GITHUB_TOKEN must be unset; run `runx env personal-shell` or `runx env scrub`" {
		t.Fatalf("failure = %q", result.Failures[0])
	}

	clean := SelfTest(map[string]string{})
	if !clean.Pass || len(clean.Failures) != 0 {
		t.Fatalf("clean SelfTest = %#v, want pass", clean)
	}
}
