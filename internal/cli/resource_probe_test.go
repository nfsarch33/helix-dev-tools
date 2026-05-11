package cli

import "testing"

func TestParseFreePct(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"The system has 51539607552 (3145728 pages with a page size of 16384). System-wide memory free percentage: 69%", 69},
		{"System-wide memory free percentage: 5%", 5},
		{"free percentage: 0", 0},
		{"some other line", -1},
		{"", -1},
	}
	for _, tc := range cases {
		got := parseFreePct(tc.in)
		if got != tc.want {
			t.Errorf("parseFreePct(%q)=%d want %d", tc.in, got, tc.want)
		}
	}
}

func TestFirstSummaryLine_PrefersFreePercentage(t *testing.T) {
	raw := `header noise
some intermediate output
The system has 12345. System-wide memory free percentage: 42%
trailing
`
	got := firstSummaryLine(raw)
	want := "The system has 12345. System-wide memory free percentage: 42%"
	if got != want {
		t.Errorf("firstSummaryLine got %q want %q", got, want)
	}
}

func TestFirstSummaryLine_FallbackFirstLine(t *testing.T) {
	raw := "first non-empty\nsecond"
	got := firstSummaryLine(raw)
	if got != "first non-empty" {
		t.Errorf("got %q want first non-empty", got)
	}
}

func TestResourceProbeCmd_Registered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"resource-probe-once"})
	if err != nil {
		t.Fatalf("rootCmd.Find(resource-probe-once): %v", err)
	}
	if cmd == nil || cmd.Use != "resource-probe-once" {
		t.Fatalf("got %#v", cmd)
	}
}
