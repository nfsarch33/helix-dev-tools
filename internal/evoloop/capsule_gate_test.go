package evoloop

import (
	"testing"
)

func TestCapsuleQualityGate_Pass(t *testing.T) {
	c := Capsule{
		Source: "wsl1",
		Kind:   KindRollup,
		Text:   "KPI improved: latency dropped 15% across fleet operations. Lesson: bounded worker pool reduces p99.",
	}
	result := CapsuleQualityGate(c)
	if !result.Pass {
		t.Errorf("expected pass, got fail: %s", result.Reason)
	}
}

func TestCapsuleQualityGate_MissingText(t *testing.T) {
	c := Capsule{
		Source: "wsl1",
		Kind:   KindRollup,
	}
	result := CapsuleQualityGate(c)
	if result.Pass {
		t.Error("capsule with empty text should fail gate")
	}
	if result.Reason != "text is empty" {
		t.Errorf("unexpected reason: %s", result.Reason)
	}
}

func TestCapsuleQualityGate_ShortText(t *testing.T) {
	c := Capsule{
		Source: "wsl1",
		Kind:   KindRollup,
		Text:   "ok",
	}
	result := CapsuleQualityGate(c)
	if result.Pass {
		t.Error("capsule with text < 20 chars should fail")
	}
}

func TestCapsuleQualityGate_MissingKind(t *testing.T) {
	c := Capsule{
		Source: "wsl1",
		Text:   "Valid summary with enough detail for the gate",
	}
	result := CapsuleQualityGate(c)
	if result.Pass {
		t.Error("capsule with empty kind should fail")
	}
}

func TestCapsuleQualityGate_MissingSource(t *testing.T) {
	c := Capsule{
		Kind: KindRollup,
		Text: "Valid summary with enough detail for the gate",
	}
	result := CapsuleQualityGate(c)
	if result.Pass {
		t.Error("capsule with empty source should fail")
	}
}
