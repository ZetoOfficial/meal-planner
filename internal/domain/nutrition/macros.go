package nutrition

import (
	"errors"
	"math"
)

type Macros struct {
	ProteinG float64
	FatG     float64
	CarbG    float64
	Kcal     int
}

func (m Macros) Valid() bool {
	return validGrams(m.ProteinG) && validGrams(m.FatG) && validGrams(m.CarbG) && m.Kcal >= 0
}

func (m Macros) Add(other Macros) (Macros, error) {
	if !m.Valid() || !other.Valid() {
		return Macros{}, errors.New("macros must be finite and nonnegative")
	}
	if other.Kcal > maxInt-m.Kcal {
		return Macros{}, errors.New("macro kcal overflow")
	}
	result := Macros{
		ProteinG: m.ProteinG + other.ProteinG,
		FatG:     m.FatG + other.FatG,
		CarbG:    m.CarbG + other.CarbG,
		Kcal:     m.Kcal + other.Kcal,
	}
	if !result.Valid() {
		return Macros{}, errors.New("macro value overflow")
	}
	return result, nil
}

// Scale preserves gram precision and rounds kcal to the nearest integer, with .5 rounded up.
func (m Macros) Scale(factor float64) (Macros, error) {
	if !m.Valid() {
		return Macros{}, errors.New("macros must be finite and nonnegative")
	}
	if math.IsNaN(factor) || math.IsInf(factor, 0) || factor < 0 {
		return Macros{}, errors.New("macro scale factor must be finite and nonnegative")
	}
	if m.Kcal > 0 && factor > float64(maxInt)/float64(m.Kcal) {
		return Macros{}, errors.New("macro kcal overflow")
	}
	result := Macros{
		ProteinG: m.ProteinG * factor,
		FatG:     m.FatG * factor,
		CarbG:    m.CarbG * factor,
		Kcal:     int(math.Round(float64(m.Kcal) * factor)),
	}
	if !result.Valid() {
		return Macros{}, errors.New("macro value overflow")
	}
	return result, nil
}

const (
	mgPerG = 1000
	maxInt = int(^uint(0) >> 1)
)

func validGrams(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0
}

func SodiumMgFromSaltG(saltG float64) float64 { return saltG / 2.54 * mgPerG }
