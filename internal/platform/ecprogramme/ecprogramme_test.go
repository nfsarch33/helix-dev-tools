package ecprogramme

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewProgramme(t *testing.T) {
	t.Parallel()
	p := New("test-programme", "base-sha")
	assert.Equal(t, "test-programme", p.Name)
	assert.Equal(t, "base-sha", p.BaseCommit)
	assert.False(t, p.StartedAt.IsZero())
	assert.Len(t, p.Batches, 0)
}

func TestAddBatch(t *testing.T) {
	t.Parallel()
	p := New("test-programme", "")
	batch := Batch{
		Name: "test-batch",
		Pairs: []Pair{
			{MVPID: "v1", QAID: "v2", Label: "test-pair"},
		},
	}
	p.AddBatch(batch)
	assert.Len(t, p.Batches, 1)
	assert.Equal(t, "test-batch", p.Batches[0].Name)
}

func TestMarkBatchDone(t *testing.T) {
	t.Parallel()
	p := New("test-programme", "")
	batch := Batch{
		Name: "test-batch",
		Pairs: []Pair{
			{MVPID: "v1", QAID: "v2", Label: "test-pair1"},
			{MVPID: "v3", QAID: "v4", Label: "test-pair2"},
		},
	}
	p.AddBatch(batch)

	err := p.MarkBatchDone("test-batch", "test-branch", "commit-sha")
	assert.NoError(t, err)
	assert.Equal(t, "test-branch", p.Batches[0].Branch)

	for _, pair := range p.Batches[0].Pairs {
		assert.True(t, pair.Done)
		assert.Equal(t, "test-branch", pair.Branch)
		assert.Equal(t, "commit-sha", pair.CommitSHA)
	}

	err = p.MarkBatchDone("nonexistent-batch", "", "")
	assert.Error(t, err)
}

func TestProgress(t *testing.T) {
	t.Parallel()
	p := New("test-programme", "")
	batch1 := Batch{
		Name: "batch1",
		Pairs: []Pair{
			{MVPID: "v1", QAID: "v2", Label: "pair1"},
			{MVPID: "v3", QAID: "v4", Label: "pair2"},
		},
	}
	batch2 := Batch{
		Name: "batch2",
		Pairs: []Pair{
			{MVPID: "v5", QAID: "v6", Label: "pair3"},
			{MVPID: "v7", QAID: "v8", Label: "pair4", Done: true},
		},
	}
	p.AddBatch(batch1).AddBatch(batch2)

	// Mark one pair in second batch as done and add a branch
	err := p.MarkBatchDone("batch2", "test-branch", "commit-sha")
	assert.NoError(t, err)

	report := p.Progress()
	assert.Equal(t, 4, report.TotalPairs)
	assert.Equal(t, 2, report.DonePairs)
	assert.Equal(t, 2, report.PendingPairs)
	assert.Equal(t, []string{"batch2"}, report.DoneBatches)
	assert.Len(t, report.PendingBranches, 1)
	assert.Equal(t, "test-branch", report.PendingBranches[0].Branch)
}

func TestFormatProgress(t *testing.T) {
	t.Parallel()
	report := ProgressReport{
		TotalPairs:    10,
		DonePairs:     5,
		PendingPairs:  5,
		DoneBatches:   []string{"batch1", "batch2"},
	}

	formattedReport := FormatProgress(report)
	assert.Contains(t, formattedReport, "Total Pairs: 10")
	assert.Contains(t, formattedReport, "Done Pairs: 5")
	assert.Contains(t, formattedReport, "Pending Pairs: 5")
	assert.Contains(t, formattedReport, "batch1")
	assert.Contains(t, formattedReport, "batch2")
}

func TestOperatorInstructions(t *testing.T) {
	t.Parallel()
	report := ProgressReport{
		PendingBranches: []BranchPushAction{
			{Branch: "branch1", CommitSHA: "sha1"},
			{Branch: "branch2", CommitSHA: "sha2"},
		},
	}

	instructions := OperatorInstructions(report, "origin")
	assert.Len(t, instructions, 2)
	assert.Equal(t, "git push origin branch1", instructions[0])
	assert.Equal(t, "git push origin branch2", instructions[1])
}

func TestLoadEC5172(t *testing.T) {
	t.Parallel()
	p := LoadEC5172()

	assert.Equal(t, "helixon-ec", p.Name)
	assert.Len(t, p.Batches, 5)

	totalPairs := 0
	for _, batch := range p.Batches {
		totalPairs += len(batch.Pairs)
	}
	assert.Equal(t, 44, totalPairs)

	for _, batch := range p.Batches {
		assert.NotEmpty(t, batch.Name)
		assert.NotEmpty(t, batch.Branch)
		assert.False(t, batch.Pairs[0].Done)

		for _, pair := range batch.Pairs {
			assert.NotEmpty(t, pair.MVPID)
			assert.NotEmpty(t, pair.QAID)
			assert.NotEmpty(t, pair.Label)
		}
	}
}