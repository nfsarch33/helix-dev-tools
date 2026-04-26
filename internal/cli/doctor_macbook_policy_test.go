package cli

import "testing"

func TestEvaluateMacbookPolicy_PassiveReplicaHealthy(t *testing.T) {
	t.Parallel()

	failures := evaluateMacbookPolicy(macbookPolicyState{
		ReplicaHealth: "healthy",
	})
	if len(failures) != 0 {
		t.Fatalf("expected no failures, got %v", failures)
	}
}

func TestEvaluateMacbookPolicy_RejectsTailscaleInstallOrProcess(t *testing.T) {
	t.Parallel()

	failures := evaluateMacbookPolicy(macbookPolicyState{
		TailscaleBinary:   true,
		TailscaleApp:      true,
		TailscaleLaunchD:  true,
		TailscaledProcess: true,
		ReplicaHealth:     "healthy",
	})
	if len(failures) != 4 {
		t.Fatalf("expected four tailscale failures, got %d: %v", len(failures), failures)
	}
}

func TestEvaluateMacbookPolicy_RequiresHealthyReplica(t *testing.T) {
	t.Parallel()

	failures := evaluateMacbookPolicy(macbookPolicyState{ReplicaHealth: "starting"})
	if len(failures) != 1 {
		t.Fatalf("expected replica health failure, got %v", failures)
	}
}

func TestDoctorMacbookPolicyCommandRegistered(t *testing.T) {
	t.Parallel()

	names := []string{}
	for _, cmd := range doctorCmd.Commands() {
		names = append(names, cmd.Name())
	}
	if !containsString(names, "macbook-policy") {
		t.Fatalf("doctor macbook-policy command not registered; got %v", names)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
