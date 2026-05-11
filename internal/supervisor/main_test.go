package supervisor

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain wraps the package test suite with goleak's leak detector so
// any rogue goroutine left over by a Service or by the supervisor
// itself fails the test run rather than silently consuming resources
// in the live daemon. See `cursor-config/rules/resource-guard.mdc`.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
