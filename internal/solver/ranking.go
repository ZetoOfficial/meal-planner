package solver

import (
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	"github.com/ZetoOfficial/meal-planner/internal/quality"
)

// scoreEpsilon keeps floating point noise from reordering otherwise equal
// candidates, which would make plan generation non-deterministic.
const scoreEpsilon = 1e-9

// dayScore is the documented lexicographic objective for one eater-day.
// Level 1 (hard safety) and level 2 (macro feasibility) are decided before a
// candidate is scored at all: infeasible candidates never reach this type.
type dayScore struct {
	// Level 3: strongest daily quality rules.
	dailySoftFails int
	dailyScore     float64
	// Level 4: rolling weekly pattern.
	rollingSoftFails int
	rollingScore     float64
	// Level 5: reuse and rotation preferences.
	reuseCount     int
	rotationWeight float64
	// Level 6: per-rule coverage.
	coverage float64
}

// betterThan compares one level at a time and never folds the levels into a
// single number, so a stronger rule outcome cannot be bought with coverage.
func (s dayScore) betterThan(other dayScore) bool {
	if s.dailySoftFails != other.dailySoftFails {
		return s.dailySoftFails < other.dailySoftFails
	}
	if higher, decided := compareFloat(s.dailyScore, other.dailyScore); decided {
		return higher
	}
	if s.rollingSoftFails != other.rollingSoftFails {
		return s.rollingSoftFails < other.rollingSoftFails
	}
	if higher, decided := compareFloat(s.rollingScore, other.rollingScore); decided {
		return higher
	}
	if s.reuseCount != other.reuseCount {
		return s.reuseCount > other.reuseCount
	}
	if higher, decided := compareFloat(s.rotationWeight, other.rotationWeight); decided {
		return higher
	}
	higher, decided := compareFloat(s.coverage, other.coverage)
	return decided && higher
}

func compareFloat(left, right float64) (higher, decided bool) {
	if left-right > scoreEpsilon {
		return true, true
	}
	if right-left > scoreEpsilon {
		return false, true
	}
	return false, false
}

func isRollingRule(code string) bool {
	switch code {
	case quality.RuleRedMeat, quality.RuleProcessedMeat, quality.RuleLegumes:
		return true
	default:
		return false
	}
}

func scoreDay(ruleset quality.Ruleset, evaluation quality.Evaluation, solution daySolution, priorUses map[int64]int) dayScore {
	daily := make([]quality.RuleOutcome, 0, len(evaluation.Outcomes))
	rolling := make([]quality.RuleOutcome, 0, len(evaluation.Outcomes))
	score := dayScore{}
	for _, outcome := range evaluation.Outcomes {
		if isRollingRule(outcome.Code) {
			rolling = append(rolling, outcome)
			if outcome.Status == nutrition.StatusSoftFail {
				score.rollingSoftFails++
			}
			continue
		}
		daily = append(daily, outcome)
		if outcome.Status == nutrition.StatusSoftFail {
			score.dailySoftFails++
		}
	}

	score.dailyScore, _ = quality.AggregateOutcomes(ruleset, daily)
	score.rollingScore, _ = quality.AggregateOutcomes(ruleset, rolling)
	score.coverage = evaluation.Coverage
	score.reuseCount, score.rotationWeight = preferenceSignals(solution, priorUses)
	return score
}

// preferenceSignals implements level 5 with the signals the domain already
// carries: reuse of batches the plan has cooked and the rotation weight of the
// selected templates. Cost, storage and 14-day novelty enter here once the
// planning use case supplies them.
func preferenceSignals(solution daySolution, priorUses map[int64]int) (reuseCount int, rotationWeight float64) {
	if len(solution.uses) == 0 {
		return 0, 0
	}
	var weightSum float64
	for i := range solution.uses {
		recipe := solution.uses[i].variant.recipe
		reuseCount += priorUses[int64(recipe.ID)]
		weightSum += recipe.Weight
	}
	return reuseCount, weightSum / float64(len(solution.uses))
}
