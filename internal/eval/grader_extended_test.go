package eval

import (
	"testing"
)

func TestParseCoverage_SinglePackage(t *testing.T) {
	output := `ok  	github.com/nfsarch33/helix-dev-tools/internal/eval	1.234s	coverage: 80.5% of statements`
	cov := parseCoverage(output)
	if cov < 80.4 || cov > 80.6 {
		t.Errorf("expected ~80.5, got %f", cov)
	}
}

func TestParseCoverage_MultiplePackages(t *testing.T) {
	output := `ok  	pkg/a	0.5s	coverage: 70.0% of statements
ok  	pkg/b	0.3s	coverage: 90.0% of statements`
	cov := parseCoverage(output)
	if cov < 79.9 || cov > 80.1 {
		t.Errorf("expected ~80.0 (avg of 70 and 90), got %f", cov)
	}
}

func TestParseCoverage_NoCoverageOutput(t *testing.T) {
	cov := parseCoverage("some random output without coverage data")
	if cov != 0 {
		t.Errorf("expected 0, got %f", cov)
	}
}

func TestCoverageGrader_Defaults(t *testing.T) {
	g := &CoverageGrader{}
	if g.Package != "" {
		t.Error("default package should be empty (resolved internally)")
	}
}

func TestTestGrader_DefaultPackage(t *testing.T) {
	g := &TestGrader{}
	if g.Package != "" {
		t.Error("default package should be empty")
	}
}

func TestVetGrader_DefaultPackage(t *testing.T) {
	g := &VetGrader{}
	if g.Package != "" {
		t.Error("default package should be empty")
	}
}

func TestTruncateGrader(t *testing.T) {
	short := "hello"
	if truncateGrader(short, 100) != "hello" {
		t.Error("should not truncate short string")
	}

	long := "abcdefghij"
	result := truncateGrader(long, 5)
	if result != "abcde...[truncated]" {
		t.Errorf("unexpected truncation: %q", result)
	}
}

func TestNewGrader_CoverageType(t *testing.T) {
	g := NewGrader(Criterion{GraderType: GraderCoverage, Package: "./...", Threshold: 80})
	if _, ok := g.(*CoverageGrader); !ok {
		t.Error("expected CoverageGrader")
	}
}

func TestNewGrader_TestType(t *testing.T) {
	g := NewGrader(Criterion{GraderType: GraderTest, Package: "./...", Race: true})
	if tg, ok := g.(*TestGrader); !ok {
		t.Error("expected TestGrader")
	} else if !tg.Race {
		t.Error("expected race=true")
	}
}

func TestNewGrader_LintType(t *testing.T) {
	g := NewGrader(Criterion{GraderType: GraderLint})
	if _, ok := g.(*LintGrader); !ok {
		t.Error("expected LintGrader")
	}
}

func TestNewGrader_VetType(t *testing.T) {
	g := NewGrader(Criterion{GraderType: GraderVet, Package: "./internal/eval/"})
	if vg, ok := g.(*VetGrader); !ok {
		t.Error("expected VetGrader")
	} else if vg.Package != "./internal/eval/" {
		t.Errorf("expected package ./internal/eval/, got %s", vg.Package)
	}
}

func TestLintGrader_WithConfig(t *testing.T) {
	g := &LintGrader{Config: ".golangci.yml"}
	if g.Config != ".golangci.yml" {
		t.Errorf("expected config, got %s", g.Config)
	}
}
