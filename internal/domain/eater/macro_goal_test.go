package eater

import (
	"math"
	"testing"
	"time"

	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
)

func TestCurrentGoalAtIgnoresFutureGoals(t *testing.T) {
	today := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	goals := []MacroGoal{
		{ID: 1, EffectiveFrom: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)},
		{ID: 2, EffectiveFrom: time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)},
		{ID: 3, EffectiveFrom: time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)},
	}

	current := CurrentGoalAt(goals, today)
	if current == nil || current.ID != 2 {
		t.Fatalf("CurrentGoalAt() = %#v, want goal 2", current)
	}
}

func TestMacroRangeContainsInclusiveBoundaries(t *testing.T) {
	goal := MacroRange{
		ProteinMinG: 100, ProteinMaxG: 120,
		FatMinG: 50, FatMaxG: 60,
		CarbMinG: 150, CarbMaxG: 180,
		KcalMin: 1800, KcalMax: 2000,
	}
	for name, macros := range map[string]nutrition.Macros{
		"minimum": {ProteinG: 100, FatG: 50, CarbG: 150, Kcal: 1800},
		"maximum": {ProteinG: 120, FatG: 60, CarbG: 180, Kcal: 2000},
	} {
		t.Run(name, func(t *testing.T) {
			if !goal.Contains(macros) {
				t.Fatalf("Contains(%#v) = false, want true", macros)
			}
		})
	}
}

func TestMacroRangeRejectsValuesOutsideAnyBoundary(t *testing.T) {
	goal := MacroRange{
		ProteinMinG: 100, ProteinMaxG: 120,
		FatMinG: 50, FatMaxG: 60,
		CarbMinG: 150, CarbMaxG: 180,
		KcalMin: 1800, KcalMax: 2000,
	}
	for name, macros := range map[string]nutrition.Macros{
		"protein": {ProteinG: 99, FatG: 55, CarbG: 165, Kcal: 1900},
		"fat":     {ProteinG: 110, FatG: 61, CarbG: 165, Kcal: 1900},
		"carb":    {ProteinG: 110, FatG: 55, CarbG: 181, Kcal: 1900},
		"kcal":    {ProteinG: 110, FatG: 55, CarbG: 165, Kcal: 2001},
	} {
		t.Run(name, func(t *testing.T) {
			if goal.Contains(macros) {
				t.Fatalf("Contains(%#v) = true, want false", macros)
			}
		})
	}
}

func TestMacroRangeRejectsInvalidRangeAndMacros(t *testing.T) {
	if (MacroRange{ProteinMinG: 2, ProteinMaxG: 1}).Contains(nutrition.Macros{}) {
		t.Fatal("invalid range contains macros")
	}
	if (MacroRange{}).Contains(nutrition.Macros{ProteinG: math.NaN()}) {
		t.Fatal("range contains invalid macros")
	}
}
