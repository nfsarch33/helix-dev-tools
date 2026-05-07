package identity

import "testing"

func TestIdentityGate_RejectsZendeskTokenFamily(t *testing.T) {
	for _, key := range TokenKeys {
		result := SelfTest(map[string]string{key: "set"})
		if result.Pass {
			t.Fatalf("SelfTest must fail when %s is set", key)
		}
		if len(result.Failures) != 1 {
			t.Fatalf("%s failures = %#v, want exactly one", key, result.Failures)
		}
	}
}
