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
