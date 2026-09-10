package nutrition

import (
	"errors"
	"math"
)

type Coverage struct {
	knownWeight float64
	totalWeight float64
}

func CoverageForInput(known bool, weight float64) (Coverage, error) {
	if math.IsNaN(weight) || math.IsInf(weight, 0) || weight < 0 {
		return Coverage{}, errors.New("coverage weight must be finite and nonnegative")
	}
	coverage := Coverage{totalWeight: weight}
	if known {
		coverage.knownWeight = weight
	}
	return coverage, nil
}

// CoverageFromScore rebuilds a coverage from the single stored score: the known
// share carries that weight and the rest is explicitly unknown, so a reloaded
// summary reports the same confidence it was saved with.
func CoverageFromScore(score float64) (Coverage, error) {
	known, err := CoverageForInput(true, score)
	if err != nil {
		return Coverage{}, err
	}
	unknown, err := CoverageForInput(false, 1-score)
	if err != nil {
		return Coverage{}, err
	}
	return known.Add(unknown), nil
}

func (c Coverage) Add(other Coverage) Coverage {
	return Coverage{
		knownWeight: c.knownWeight + other.knownWeight,
		totalWeight: c.totalWeight + other.totalWeight,
	}
}

func (c Coverage) Score() float64 {
	if c.totalWeight == 0 {
		return 0
	}
	return c.knownWeight / c.totalWeight
}
