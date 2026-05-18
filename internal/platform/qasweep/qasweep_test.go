package qasweep

import "testing"

func TestRepoResult_Overall_Pass(t *testing.T) {
	r := RepoResult{RepoName: "helixon", SentruxPass: true, TestPass: true, ShellLeak: false}
	if !r.Overall() {
		t.Error("expected overall pass")
	}
}

func TestRepoResult_Overall_Fail(t *testing.T) {
	r := RepoResult{RepoName: "bad", SentruxPass: false, TestPass: true, ShellLeak: false}
	if r.Overall() {
		t.Error("expected overall fail when sentrux fails")
	}
	r2 := RepoResult{SentruxPass: true, TestPass: true, ShellLeak: true}
	if r2.Overall() {
		t.Error("expected overall fail when shell leak detected")
	}
}

func TestReport_AllPassed(t *testing.T) {
	rep := NewReport()
	rep.Add(RepoResult{RepoName: "a", SentruxPass: true, TestPass: true})
	rep.Add(RepoResult{RepoName: "b", SentruxPass: true, TestPass: true})
	if !rep.AllPassed() {
		t.Error("expected all passed")
	}
}

func TestReport_FailedRepos(t *testing.T) {
	rep := NewReport()
	rep.Add(RepoResult{RepoName: "good", SentruxPass: true, TestPass: true})
	rep.Add(RepoResult{RepoName: "bad1", SentruxPass: false, TestPass: true})
	rep.Add(RepoResult{RepoName: "bad2", SentruxPass: true, TestPass: false})
	failed := rep.FailedRepos()
	if len(failed) != 2 {
		t.Errorf("expected 2 failed repos, got %v", failed)
	}
}

func TestReport_PassCount(t *testing.T) {
	rep := NewReport()
	rep.Add(RepoResult{RepoName: "pass1", SentruxPass: true, TestPass: true})
	rep.Add(RepoResult{RepoName: "pass2", SentruxPass: true, TestPass: true})
	rep.Add(RepoResult{RepoName: "fail1", SentruxPass: false, TestPass: true})
	if rep.PassCount() != 2 {
		t.Errorf("expected 2 passing repos, got %d", rep.PassCount())
	}
}

func TestReport_Empty_AllPassed(t *testing.T) {
	rep := NewReport()
	if !rep.AllPassed() {
		t.Error("empty report should be all-passed")
	}
}
