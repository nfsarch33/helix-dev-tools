package ecprogramme

import (
	"fmt"
	"strings"
	"time"
)

// Pair represents one sprint pair (MVP + QA) in the EC programme.
type Pair struct {
	MVPID     string // e.g. "v5172"
	QAID      string // e.g. "v5173"
	Label     string // human label e.g. "OAuth2 provider"
	Done      bool
	Branch    string // git branch name if implemented
	CommitSHA string
}

// Batch groups related pairs into a named batch.
type Batch struct {
	Name   string
	Pairs  []Pair
	Branch string // single branch covers all pairs in a batch
}

// Programme holds the full EC programme definition and state.
type Programme struct {
	Name       string
	BaseCommit string
	Batches    []Batch
	StartedAt  time.Time
}

// New creates a Programme with the given name and base commit.
func New(name, baseCommit string) *Programme {
	return &Programme{
		Name:       name,
		BaseCommit: baseCommit,
		StartedAt:  time.Now(),
	}
}

// AddBatch appends a batch to the programme.
func (p *Programme) AddBatch(b Batch) *Programme {
	p.Batches = append(p.Batches, b)
	return p
}

// MarkBatchDone marks all pairs in a batch as done, setting branch and commit.
func (p *Programme) MarkBatchDone(batchName, branch, commitSHA string) error {
	for i, batch := range p.Batches {
		if batch.Name == batchName {
			p.Batches[i].Branch = branch
			for j := range p.Batches[i].Pairs {
				p.Batches[i].Pairs[j].Done = true
				p.Batches[i].Pairs[j].Branch = branch
				p.Batches[i].Pairs[j].CommitSHA = commitSHA
			}
			return nil
		}
	}
	return fmt.Errorf("batch %s not found", batchName)
}

// Progress returns a ProgressReport.
type ProgressReport struct {
	TotalPairs       int
	DonePairs        int
	PendingPairs     int
	DoneBatches      []string
	PendingBranches  []BranchPushAction
}

type BranchPushAction struct {
	Branch    string
	CommitSHA string
	PRTitle   string
}

func (p *Programme) Progress() ProgressReport {
	report := ProgressReport{}

	for _, batch := range p.Batches {
		report.TotalPairs += len(batch.Pairs)

		batchDone := true
		var pendingBranchAction BranchPushAction

		for _, pair := range batch.Pairs {
			if pair.Done {
				report.DonePairs++
			} else {
				report.PendingPairs++
				batchDone = false
			}
		}

		if batchDone && batch.Branch != "" {
			report.DoneBatches = append(report.DoneBatches, batch.Name)
			pendingBranchAction = BranchPushAction{
				Branch:    batch.Branch,
				CommitSHA: batch.Pairs[0].CommitSHA,
				PRTitle:   fmt.Sprintf("Deliver %s", batch.Name),
			}
			report.PendingBranches = append(report.PendingBranches, pendingBranchAction)
		}
	}

	return report
}

// FormatProgress returns a Markdown progress report.
func FormatProgress(r ProgressReport) string {
	var sb strings.Builder
	sb.WriteString("# EC Programme Progress\n\n")
	sb.WriteString(fmt.Sprintf("Total Pairs: %d\n", r.TotalPairs))
	sb.WriteString(fmt.Sprintf("Done Pairs: %d\n", r.DonePairs))
	sb.WriteString(fmt.Sprintf("Pending Pairs: %d\n\n", r.PendingPairs))

	if len(r.DoneBatches) > 0 {
		sb.WriteString("## Completed Batches\n")
		for _, batch := range r.DoneBatches {
			sb.WriteString(fmt.Sprintf("- %s\n", batch))
		}
	}

	return sb.String()
}

// OperatorInstructions returns shell commands to push all pending branches.
func OperatorInstructions(r ProgressReport, remote string) []string {
	var commands []string
	for _, branch := range r.PendingBranches {
		cmd := fmt.Sprintf("git push %s %s", remote, branch.Branch)
		commands = append(commands, cmd)
	}
	return commands
}

// LoadEC5172 returns a pre-populated Programme for the helixon-ec v5172-v5261 programme.
func LoadEC5172() *Programme {
	p := New("helixon-ec", "")

	batches := []Batch{
		{
			Name:   "batch6",
			Branch: "feat/v5172-v5189-batch6-platform-integration",
			Pairs: []Pair{
				{MVPID: "v5172", QAID: "v5173", Label: "oauth2"},
				{MVPID: "v5174", QAID: "v5175", Label: "inbound-webhook"},
				{MVPID: "v5176", QAID: "v5177", Label: "key-management"},
				{MVPID: "v5178", QAID: "v5179", Label: "user-profile"},
				{MVPID: "v5180", QAID: "v5181", Label: "connection-pool"},
				{MVPID: "v5182", QAID: "v5183", Label: "config-provider"},
				{MVPID: "v5184", QAID: "v5185", Label: "logging"},
				{MVPID: "v5186", QAID: "v5187", Label: "telemetry"},
				{MVPID: "v5188", QAID: "v5189", Label: "feature-flags"},
			},
		},
		{
			Name:   "batch7",
			Branch: "feat/v5190-v5207-batch7-business-logic",
			Pairs: []Pair{
				{MVPID: "v5190", QAID: "v5191", Label: "pricing"},
				{MVPID: "v5192", QAID: "v5193", Label: "inventory"},
				{MVPID: "v5194", QAID: "v5195", Label: "cart"},
				{MVPID: "v5196", QAID: "v5197", Label: "checkout"},
				{MVPID: "v5198", QAID: "v5199", Label: "order-pipeline"},
				{MVPID: "v5200", QAID: "v5201", Label: "payment-integration"},
				{MVPID: "v5202", QAID: "v5203", Label: "refund-process"},
				{MVPID: "v5204", QAID: "v5205", Label: "analytics"},
				{MVPID: "v5206", QAID: "v5207", Label: "reporting"},
			},
		},
		{
			Name:   "batch8",
			Branch: "feat/v5208-v5225-batch8-operations",
			Pairs: []Pair{
				{MVPID: "v5208", QAID: "v5209", Label: "invsync"},
				{MVPID: "v5210", QAID: "v5211", Label: "orderrouting"},
				{MVPID: "v5212", QAID: "v5213", Label: "returnlabel"},
				{MVPID: "v5214", QAID: "v5215", Label: "shiplabel"},
				{MVPID: "v5216", QAID: "v5217", Label: "pkgtrack"},
				{MVPID: "v5220", QAID: "v5221", Label: "taxexempt"},
				{MVPID: "v5222", QAID: "v5223", Label: "invoicegen"},
				{MVPID: "v5224", QAID: "v5225", Label: "creditnote"},
			},
		},
		{
			Name:   "batch9",
			Branch: "feat/v5226-v5243-batch9-analytics",
			Pairs: []Pair{
				{MVPID: "v5226", QAID: "v5227", Label: "event-tracking"},
				{MVPID: "v5228", QAID: "v5229", Label: "user-segment"},
				{MVPID: "v5230", QAID: "v5231", Label: "conversion-funnel"},
				{MVPID: "v5232", QAID: "v5233", Label: "cohort-analysis"},
				{MVPID: "v5234", QAID: "v5235", Label: "a-b-testing"},
				{MVPID: "v5236", QAID: "v5237", Label: "machine-learning"},
				{MVPID: "v5238", QAID: "v5239", Label: "predictive-model"},
				{MVPID: "v5240", QAID: "v5241", Label: "recommendation"},
				{MVPID: "v5242", QAID: "v5243", Label: "dashboard"},
			},
		},
		{
			Name:   "batch10",
			Branch: "feat/v5244-v5261-batch10-hardening",
			Pairs: []Pair{
				{MVPID: "v5244", QAID: "v5245", Label: "performance"},
				{MVPID: "v5246", QAID: "v5247", Label: "scalability"},
				{MVPID: "v5248", QAID: "v5249", Label: "stress-test"},
				{MVPID: "v5250", QAID: "v5251", Label: "fuzzing"},
				{MVPID: "v5252", QAID: "v5253", Label: "pen-testing"},
				{MVPID: "v5254", QAID: "v5255", Label: "vulncheck"},
				{MVPID: "v5256", QAID: "v5257", Label: "hardening"},
				{MVPID: "v5258", QAID: "v5259", Label: "recovery"},
				{MVPID: "v5260", QAID: "v5261", Label: "chaos"},
			},
		},
	}

	for _, batch := range batches {
		p.AddBatch(batch)
	}

	return p
}