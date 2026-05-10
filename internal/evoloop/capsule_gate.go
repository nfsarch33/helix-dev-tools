package evoloop

// QualityGateResult is the outcome of a capsule content quality check.
// The gate enforces minimum content standards before a capsule can be
// promoted to a durable rule, skill, or KB entry.
type QualityGateResult struct {
	Pass   bool
	Reason string
}

const minTextLength = 20

// CapsuleQualityGate validates that a capsule meets minimum content
// standards for promotion. Returns a pass/fail with reason.
func CapsuleQualityGate(c Capsule) QualityGateResult {
	if c.Kind == "" {
		return QualityGateResult{Pass: false, Reason: "kind is empty"}
	}
	if c.Source == "" {
		return QualityGateResult{Pass: false, Reason: "source is empty"}
	}
	if c.Text == "" {
		return QualityGateResult{Pass: false, Reason: "text is empty"}
	}
	if len(c.Text) < minTextLength {
		return QualityGateResult{Pass: false, Reason: "text too short (< 20 chars)"}
	}
	return QualityGateResult{Pass: true, Reason: ""}
}
