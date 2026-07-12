package eater

import "time"

type MacroRange struct {
	ProteinMinG, ProteinMaxG float64
	FatMinG, FatMaxG         float64
	CarbMinG, CarbMaxG       float64
	KcalMin, KcalMax         int
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
	date = date.UTC().Truncate(24 * time.Hour)
	var current *MacroGoal
	for i := range goals {
		goal := goals[i]
		if goal.EffectiveFrom.UTC().Truncate(24 * time.Hour).After(date) {
			continue
		}
		if current == nil || goal.EffectiveFrom.After(current.EffectiveFrom) {
			current = &goal
		}
	}
	return current
}
