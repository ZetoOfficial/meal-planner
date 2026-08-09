package nutrition

import (
	"math"
	"testing"
)

func TestCoverageForKnownAndUnknownInputs(t *testing.T) {
	known, err := CoverageForInput(true, 100)
	if err != nil {
		t.Fatalf("CoverageForInput(known) error = %v", err)
	}
	unknown, err := CoverageForInput(false, 50)
	if err != nil {
		t.Fatalf("CoverageForInput(unknown) error = %v", err)
	}

	if got, want := known.Add(unknown).Score(), 2.0/3.0; got != want {
		t.Fatalf("combined coverage = %v, want %v", got, want)
	}
	if got := known.Score(); got != 1 {
		t.Fatalf("known input coverage = %v, want 1", got)
	}
}

func TestCoverageWithoutRelevantInputsIsUnknown(t *testing.T) {
	if got := (Coverage{}).Score(); got != 0 {
		t.Fatalf("empty coverage = %v, want 0", got)
	}
}

func TestCoverageRejectsInvalidWeight(t *testing.T) {
	for name, weight := range map[string]float64{
		"negative":     -1,
		"not a number": math.NaN(),
		"infinite":     math.Inf(1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := CoverageForInput(true, weight); err == nil {
				t.Fatal("CoverageForInput() error = nil, want invalid weight error")
			}
		})
	}
}
