package housekeeping

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMergedBranchesExtractsBranchNames(t *testing.T) {
	input := `  remotes/origin/feat/v420-payment-foundation
  remotes/origin/fix/v530-sentrux-complexity
  remotes/origin/release/v500-prep`

	branches := ParseMergedBranches(input)
	require.Len(t, branches, 3)
	assert.Equal(t, "feat/v420-payment-foundation", branches[0])
	assert.Equal(t, "fix/v530-sentrux-complexity", branches[1])
	assert.Equal(t, "release/v500-prep", branches[2])
}

func TestParseMergedBranchesSkipsMainAndHead(t *testing.T) {
	input := `  remotes/origin/main
  remotes/origin/HEAD -> origin/main
  remotes/origin/feat/v420-payment`

	branches := ParseMergedBranches(input)
	require.Len(t, branches, 1)
	assert.Equal(t, "feat/v420-payment", branches[0])
}

func TestParseGoneBranchesExtractsBranchNames(t *testing.T) {
	input := `  feat/v5010-marketplace-sync 9438374 [origin/feat/v5010-marketplace-sync: gone] feat(marketplace): wire
  feat/v5017-temporal-hardening-qa c022551 [origin/feat/v5017-temporal-hardening-qa: gone] fix(workflows): preserve`

	branches := ParseGoneBranches(input)
	require.Len(t, branches, 2)
	assert.Equal(t, "feat/v5010-marketplace-sync", branches[0])
	assert.Equal(t, "feat/v5017-temporal-hardening-qa", branches[1])
}

func TestParseGoneBranchesSkipsActiveAndTracked(t *testing.T) {
	input := `* main abc1234 [origin/main] latest commit
  feat/active def5678 [origin/feat/active] tracked and present`

	branches := ParseGoneBranches(input)
	assert.Empty(t, branches)
}

func TestClassifyFindingCountsCorrectly(t *testing.T) {
	findings := []Finding{
		{Repo: "a", Type: "merged_remote", Branch: "feat/old"},
		{Repo: "a", Type: "gone_local", Branch: "feat/stale"},
		{Repo: "b", Type: "merged_remote", Branch: "fix/done"},
	}

	summary := ClassifyFindings(findings)
	assert.Equal(t, 2, summary.MergedRemote)
	assert.Equal(t, 1, summary.GoneLocal)
	assert.Equal(t, 2, summary.RepoCount)
}
