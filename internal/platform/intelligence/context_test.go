package intelligence

import "testing"

func TestContextWindow_Add(t *testing.T) {
	w := NewContextWindow(100)
	ok := w.Add("user", "hello world")
	if !ok {
		t.Error("expected add to succeed")
	}
	if w.UsedTokens() == 0 {
		t.Error("expected non-zero token usage")
	}
}

func TestContextWindow_Overflow(t *testing.T) {
	w := NewContextWindow(5)
	ok := w.Add("user", "this is a sentence with many words that exceeds the limit")
	if ok {
		t.Error("expected add to fail when exceeding max tokens")
	}
}

func TestContextWindow_RemainingTokens(t *testing.T) {
	w := NewContextWindow(1000)
	w.Add("user", "short")
	remaining := w.RemainingTokens()
	if remaining >= 1000 {
		t.Error("expected remaining to decrease after add")
	}
}

func TestContextWindow_Truncate(t *testing.T) {
	w := NewContextWindow(10000)
	w.Add("user", "first")
	w.Add("assistant", "second")
	w.Add("user", "third")

	w.Truncate(2)
	entries := w.Entries()
	if len(entries) != 2 {
		t.Errorf("expected 2 entries after truncate, got %d", len(entries))
	}
}

func TestContextWindow_Clear(t *testing.T) {
	w := NewContextWindow(1000)
	w.Add("user", "data")
	w.Clear()
	if w.UsedTokens() != 0 {
		t.Error("expected 0 tokens after clear")
	}
}

func TestEstimateTokens(t *testing.T) {
	tokens := EstimateTokens("one two three four")
	if tokens < 4 || tokens > 10 {
		t.Errorf("unexpected token estimate: %d", tokens)
	}
}
