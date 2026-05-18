package qualitybadge

import "testing"

func TestCompute_Thresholds(t *testing.T) {
	cases := []struct {
		score float64
		want  Badge
	}{
		{0.95, BadgePlatinum},
		{0.90, BadgePlatinum},
		{0.85, BadgeGold},
		{0.75, BadgeGold},
		{0.70, BadgeSilver},
		{0.60, BadgeSilver},
		{0.59, BadgeBronze},
		{0.0, BadgeBronze},
	}
	for _, tc := range cases {
		got := Compute(tc.score)
		if got != tc.want {
			t.Errorf("Compute(%.2f) = %s, want %s", tc.score, got, tc.want)
		}
	}
}

func TestHistory_Record_Latest(t *testing.T) {
	h := NewHistory()
	h.Record(BadgeRecord{RepoID: "helixon", Badge: BadgeSilver, Score: 0.65})
	h.Record(BadgeRecord{RepoID: "helixon", Badge: BadgeGold, Score: 0.80})
	latest, ok := h.Latest("helixon")
	if !ok {
		t.Fatal("expected latest to be found")
	}
	if latest.Badge != BadgeGold {
		t.Errorf("expected latest badge to be gold, got %s", latest.Badge)
	}
}

func TestHistory_Latest_NotFound(t *testing.T) {
	h := NewHistory()
	_, ok := h.Latest("missing")
	if ok {
		t.Error("expected false for missing repo")
	}
}

func TestHistory_AllForRepo(t *testing.T) {
	h := NewHistory()
	h.Record(BadgeRecord{RepoID: "a", Badge: BadgeBronze, Score: 0.5})
	h.Record(BadgeRecord{RepoID: "b", Badge: BadgeSilver, Score: 0.65})
	h.Record(BadgeRecord{RepoID: "a", Badge: BadgeSilver, Score: 0.70})
	all := h.AllForRepo("a")
	if len(all) != 2 {
		t.Errorf("expected 2 records for repo a, got %d", len(all))
	}
}

func TestHistory_Regressed(t *testing.T) {
	h := NewHistory()
	h.Record(BadgeRecord{RepoID: "x", Score: 0.85})
	h.Record(BadgeRecord{RepoID: "x", Score: 0.65})
	if !h.Regressed("x") {
		t.Error("expected regression detected")
	}
}

func TestHistory_NotRegressed(t *testing.T) {
	h := NewHistory()
	h.Record(BadgeRecord{RepoID: "x", Score: 0.65})
	h.Record(BadgeRecord{RepoID: "x", Score: 0.85})
	if h.Regressed("x") {
		t.Error("expected no regression when score improved")
	}
}
