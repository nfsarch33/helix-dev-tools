package mcpfilter_test

import (
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/mcpfilter"
)

func TestSoak_AllProfilesSurvive100Sessions(t *testing.T) {
	t.Parallel()
	cfg := sampleConfig()
	profiles := mcpfilter.ListProfiles()
	if len(profiles) < 7 {
		t.Fatalf("need >= 7 profiles, got %d", len(profiles))
	}

	for _, profile := range profiles {
		profile := profile
		t.Run(profile.Name, func(t *testing.T) {
			t.Parallel()
			for session := 0; session < 100; session++ {
				filtered, result := mcpfilter.ApplyProfile(cfg, profile)
				if result.TotalOut == 0 && result.TotalIn > 0 {
					t.Fatalf("session %d: profile %q produced 0 servers from %d input",
						session, profile.Name, result.TotalIn)
				}
				if result.TotalOut > result.TotalIn {
					t.Fatalf("session %d: profile %q output %d > input %d",
						session, profile.Name, result.TotalOut, result.TotalIn)
				}
				if result.ReductionPc < 0 || result.ReductionPc > 100 {
					t.Fatalf("session %d: profile %q reduction %.1f%% out of range",
						session, profile.Name, result.ReductionPc)
				}
				_ = filtered
			}
		})
	}
}

func TestSoak_ProfileSavingsConsistent(t *testing.T) {
	t.Parallel()
	cfg := sampleConfig()

	expectedMinReduction := map[string]float64{
		"research":    50,
		"code-review": 70,
		"deployment":  80,
		"debug":       60,
		"writing":     60,
		"job-hunt":    60,
		"minimal":     80,
	}

	for name, minPc := range expectedMinReduction {
		name, minPc := name, minPc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			profile, ok := mcpfilter.GetProfile(name)
			if !ok {
				t.Fatalf("profile %q not found", name)
			}
			_, result := mcpfilter.ApplyProfile(cfg, profile)
			if result.ReductionPc < minPc {
				t.Errorf("profile %q: reduction %.1f%%, want >= %.1f%%",
					name, result.ReductionPc, minPc)
			}
		})
	}
}

func TestSoak_ProfileIdempotent(t *testing.T) {
	t.Parallel()
	cfg := sampleConfig()
	profiles := mcpfilter.ListProfiles()

	for _, profile := range profiles {
		profile := profile
		t.Run(profile.Name, func(t *testing.T) {
			t.Parallel()
			first, r1 := mcpfilter.ApplyProfile(cfg, profile)
			_, r2 := mcpfilter.ApplyProfile(first, profile)
			if r2.TotalOut != r1.TotalOut {
				t.Errorf("profile %q not idempotent: first=%d, second=%d",
					profile.Name, r1.TotalOut, r2.TotalOut)
			}
		})
	}
}
