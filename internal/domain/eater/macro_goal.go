package eater

import (
	"math"
	"time"

	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
)

const dayDuration = 24 * time.Hour

type MacroRange struct {
	ProteinMinG, ProteinMaxG float64
	FatMinG, FatMaxG         float64
	CarbMinG, CarbMaxG       float64
	KcalMin, KcalMax         int
}

func (r MacroRange) Valid() bool {
	return validRange(r.ProteinMinG, r.ProteinMaxG) &&
		validRange(r.FatMinG, r.FatMaxG) &&
		validRange(r.CarbMinG, r.CarbMaxG) &&
		r.KcalMin >= 0 && r.KcalMin <= r.KcalMax
}

func (r MacroRange) Contains(macros nutrition.Macros) bool {
	return r.Valid() && macros.Valid() &&
		macros.ProteinG >= r.ProteinMinG && macros.ProteinG <= r.ProteinMaxG &&
		macros.FatG >= r.FatMinG && macros.FatG <= r.FatMaxG &&
		macros.CarbG >= r.CarbMinG && macros.CarbG <= r.CarbMaxG &&
		macros.Kcal >= r.KcalMin && macros.Kcal <= r.KcalMax
}

func validRange(minimum, maximum float64) bool {
	return !math.IsNaN(minimum) && !math.IsInf(minimum, 0) &&
		!math.IsNaN(maximum) && !math.IsInf(maximum, 0) &&
		minimum >= 0 && minimum <= maximum
}

type MacroGoalID int64

type MacroGoal struct {
	ID            MacroGoalID
	EaterID       ID
	EffectiveFrom time.Time
	Range         MacroRange
	Note          *string
}

// CurrentGoalAt returns the most recently effective goal on the supplied date.
// Future versions remain in the history but must not change current planning.
func CurrentGoalAt(goals []MacroGoal, date time.Time) *MacroGoal {
	date = date.UTC().Truncate(dayDuration)
	var current *MacroGoal
	for i := range goals {
		goal := goals[i]
		if goal.EffectiveFrom.UTC().Truncate(dayDuration).After(date) {
			continue
		}
		if current == nil || goal.EffectiveFrom.After(current.EffectiveFrom) {
			current = &goal
		}
	}
	return current
}
