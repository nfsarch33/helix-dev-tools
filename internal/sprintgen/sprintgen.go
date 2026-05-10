package sprintgen

import (
	"fmt"
	"strings"
)

// Scaffold generates a Markdown sprint scaffold for the given sprint ID,
// type (MVP/QA), and theme. The output follows the Universal Story Scaffold
// defined in the backlog roadmap: 5 themed stories + 2 universal stories
// (Hygiene KPI + EvoLoop capsule/retro).
func Scaffold(sprintID, sprintType, theme string) string {
	if sprintID == "" {
		return ""
	}

	var b strings.Builder

	fmt.Fprintf(&b, "## %s (%s) -- %s\n\n", sprintID, sprintType, theme)

	for i := 1; i <= 5; i++ {
		fmt.Fprintf(&b, "- **%s-%d**: [TODO: themed story %d]\n", sprintID, i, i)
		fmt.Fprintf(&b, "  - **OSS evidence (R1)**: [required before MVP/QA-Retro]\n")
	}

	b.WriteString(fmt.Sprintf("- **%s-6**: Hygiene KPI\n", sprintID))
	b.WriteString(fmt.Sprintf("- **%s-7**: EvoLoop capsule + retro\n", sprintID))

	return b.String()
}
