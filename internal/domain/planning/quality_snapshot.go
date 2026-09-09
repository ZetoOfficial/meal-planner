package planning

import (
	"time"

	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	mealdom "github.com/ZetoOfficial/meal-planner/internal/domain/meal"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
)

// RuleEntityType names what a stored rule verdict was computed for. It mirrors
// the database enum so a warning can be traced back to its evaluated entity.
type RuleEntityType string

const (
	RuleEntityMeal          RuleEntityType = "meal"
	RuleEntityDay           RuleEntityType = "day"
	RuleEntityRollingWindow RuleEntityType = "rolling_window"
	RuleEntityPlan          RuleEntityType = "plan"
)

// QualitySnapshot freezes one eater-day evaluation of a plan: the day summary,
// the rolling projection it was ranked with, and the rule verdicts behind both.
// Snapshots are identified by plan, eater, date and basis, so regenerating a
// plan replaces its own snapshot without touching another plan's.
type QualitySnapshot struct {
	EaterID      eaterdom.ID
	Date         time.Time
	Basis        nutrition.SummaryBasis
	Day          nutrition.DaySummary
	Rolling      nutrition.RollingSummary
	DayRules     []nutrition.RuleEvaluationResult
	RollingRules []nutrition.RuleEvaluationResult
}

// WindowStart is the first day covered by the rolling projection stored with
// this snapshot.
func (s QualitySnapshot) WindowStart() time.Time {
	if s.Rolling.WindowDays <= 1 {
		return mealdom.DayOf(s.Date)
	}
	return mealdom.DayOf(s.Date).AddDate(0, 0, -(s.Rolling.WindowDays - 1))
}

// BasisOrPlanned defaults an unset basis to the planned snapshot.
func (s QualitySnapshot) BasisOrPlanned() nutrition.SummaryBasis {
	if s.Basis == nutrition.SummaryBasisActual {
		return nutrition.SummaryBasisActual
	}
	return nutrition.SummaryBasisPlanned
}
