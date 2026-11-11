package solver

import (
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
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
	// Level 5: the eater's soft exclusions, then reuse, rotation, cost and
	// storage effort.
	softExclusionHits int
	newBatches        int
	reuseCount        int
	rotationWeight    float64
	cost              float64
	reheatedBatches   int
	// Level 6: per-rule coverage.
	coverage float64
}

// betterThan compares one level at a time and never folds the levels into a
// single number, so a stronger rule outcome cannot be bought with coverage.
func (s dayScore) betterThan(other dayScore) bool {
	if better, decided := s.compareQuality(other); decided {
		return better
	}
	return s.preferredOver(other)
}

// betterQualityThan compares only the quality rules (levels 1–4). The novelty
// dial uses it to tell a novel day that costs nutrition from one that is free.
func (s dayScore) betterQualityThan(other dayScore) bool {
	better, decided := s.compareQuality(other)
	return decided && better
}

func (s dayScore) compareQuality(other dayScore) (better, decided bool) {
	if s.dailySoftFails != other.dailySoftFails {
		return s.dailySoftFails < other.dailySoftFails, true
	}
	if higher, decided := compareFloat(s.dailyScore, other.dailyScore); decided {
		return higher, true
	}
	if s.rollingSoftFails != other.rollingSoftFails {
		return s.rollingSoftFails < other.rollingSoftFails, true
	}
	return compareFloat(s.rollingScore, other.rollingScore)
}

// preferredOver breaks a quality tie with level 5 and then coverage.
func (s dayScore) preferredOver(other dayScore) bool {
	if s.softExclusionHits != other.softExclusionHits {
		return s.softExclusionHits < other.softExclusionHits
	}
	if s.newBatches != other.newBatches {
		return s.newBatches < other.newBatches
	}
	if s.reuseCount != other.reuseCount {
		return s.reuseCount > other.reuseCount
	}
	if higher, decided := compareFloat(s.rotationWeight, other.rotationWeight); decided {
		return higher
	}
	if higher, decided := compareFloat(s.cost, other.cost); decided {
		return !higher
	}
	if s.reheatedBatches != other.reheatedBatches {
		return s.reheatedBatches < other.reheatedBatches
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

func scoreDay(ruleset quality.Ruleset, evaluation quality.Evaluation, solution daySolution, preferences dayPreferences) dayScore {
	daily := make([]quality.RuleOutcome, 0, len(evaluation.Outcomes))
	rolling := make([]quality.RuleOutcome, 0, len(evaluation.Outcomes))
	score := dayScore{}
	for _, outcome := range evaluation.Outcomes {
		if quality.IsRollingRule(outcome.Code) {
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
	score.applyPreferences(solution, preferences)
	return score
}

// applyPreferences implements level 5. Every signal is a tie-break below the
// quality rules, so none of them can buy a worse nutrition outcome:
//
//   - soft exclusions: products the eater asked to see less often;
//   - new batches: a portion of a batch the plan already cooks is free, a new
//     batch is one more thing to cook (this is what keeps a plan near the 5–7
//     components the household can actually prepare);
//   - reuse and rotation weight, or the weighted random key at novelty 1;
//   - cost of the priced ingredients;
//   - storage effort: batches that need reheating rank after those that do not.
func (s *dayScore) applyPreferences(solution daySolution, preferences dayPreferences) {
	if len(solution.uses) == 0 {
		return
	}
	var weightSum float64
	counted := make(map[int]struct{}, len(solution.uses))
	for i := range solution.uses {
		use := &solution.uses[i]
		recipe := &use.variant.recipe
		scale := use.servingUnits / float64(recipe.BaseServings)

		s.reuseCount += preferences.priorUses[int64(recipe.ID)]
		weightSum += rotationSignal(*recipe, preferences)
		for j := range use.variant.slots {
			slot := &use.variant.slots[j]
			if _, excluded := preferences.softExcluded[slot.productID]; excluded {
				s.softExclusionHits++
			}
			if slot.pricePer100G != nil {
				s.cost += *slot.pricePer100G * slot.baseQtyG * scale / gramsPer100
			}
		}

		// A variant used in two meals of the day is still one batch.
		key := variantKey(solution.uses, i)
		if _, seen := counted[key]; seen {
			continue
		}
		counted[key] = struct{}{}
		if preferences.planned == nil || !preferences.planned(use.variant) {
			s.newBatches++
		}
		if recipe.Storage == recipedom.StorageKeeps3dReheat {
			s.reheatedBatches++
		}
	}
	s.rotationWeight = weightSum / float64(len(solution.uses))
}

func rotationSignal(recipe recipedom.Template, preferences dayPreferences) float64 {
	if preferences.shuffleKeys != nil {
		return preferences.shuffleKeys[recipe.ID]
	}
	return recipe.Weight
}

// variantKey returns the index of the first use that cooks the same batch, so
// equal variants share one key without hashing their fills.
func variantKey(uses []componentUse, index int) int {
	for i := range index {
		if uses[i].variant.recipe.ID == uses[index].variant.recipe.ID &&
			sameFills(uses[i].variant.baseFills, uses[index].variant.baseFills) {
			return i
		}
	}
	return index
}
