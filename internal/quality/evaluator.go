package quality

import (
	"fmt"
	"math"

	"github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
)

const (
	kcalPerGramSugar     = 4.0
	kcalPerGramFat       = 9.0
	scoreScalePercent    = 100.0
	warnScore            = 0.5
	legumeServingsTarget = 3.0
)

// HardViolation represents a hard safety rejection, separate from soft outcomes.
type HardViolation struct {
	RuleCode string
	Message  string
}

// RuleOutcome is the result of evaluating one quality rule.
type RuleOutcome struct {
	Code     string
	Status   nutrition.RuleEvalStatus
	Severity RuleSeverity
	Actual   *float64
	Target   *float64
	Unit     string
	Coverage float64
	Score    float64
	Message  string
}

// Evaluation aggregates soft rule outcomes, the quality score, and coverage.
type Evaluation struct {
	Outcomes []RuleOutcome
	Score    float64
	Coverage float64
}

// PlanInput carries plan-level facts needed by the evaluator.
type PlanInput struct {
	DistinctLegumeComponents int
}

// DetectHardViolations scans the raw day input for hard safety rejections.
// In MVP this covers only explicit industrial trans fats (PHO or a positive
// industrial_trans_fat_per_100g value). Allergens and hard eater exclusions are
// solver constraints, not quality-rule hard violations.
func DetectHardViolations(input DayInput) []HardViolation {
	var violations []HardViolation
	for _, meal := range input.Meals {
		if meal.Fixed != nil {
			continue
		}
		for _, comp := range meal.Components {
			for i := range comp.Slots {
				if v := detectSlotHardViolation(&comp.Slots[i]); v != nil {
					violations = append(violations, *v)
				}
			}
		}
	}
	return violations
}

func detectSlotHardViolation(slot *SlotInput) *HardViolation {
	facts := slot.Facts
	if facts.ContainsPartiallyHydrogenatedOils != nil && *facts.ContainsPartiallyHydrogenatedOils {
		return &HardViolation{RuleCode: RuleIndustrialTransFat, Message: "product contains partially hydrogenated oils"}
	}
	if facts.IndustrialTransFatPer100G != nil && *facts.IndustrialTransFatPer100G > 0 {
		return &HardViolation{
			RuleCode: RuleIndustrialTransFat,
			Message:  fmt.Sprintf("industrial trans fat %.2f g/100g", *facts.IndustrialTransFatPer100G),
		}
	}
	return nil
}

// Evaluate computes the full MVP quality evaluation for a day, optional rolling
// window, and optional plan-level input. A nil rolling summary skips the weekly
// rules; a nil PlanInput skips the distinct-component legume condition.
func Evaluate(ruleset Ruleset, sex eater.Sex, day nutrition.DaySummary, rolling *nutrition.RollingSummary, plan *PlanInput) Evaluation {
	ev := Evaluation{}

	ev.Outcomes = append(
		ev.Outcomes,
		evalFiber(ruleset, sex, day),
		evalSodium(ruleset, day),
		evalAddedSugar(ruleset, day),
		evalSatFat(ruleset, day),
		evalIndustrialTransFat(ruleset, day),
		evalFruitVeg(ruleset, day),
		evalWholeGrain(ruleset, day),
		evalUPF(ruleset, day),
	)

	if rolling != nil {
		ev.Outcomes = append(
			ev.Outcomes,
			evalRedMeat(ruleset, *rolling),
			evalProcessedMeat(ruleset, *rolling),
			evalLegumes(ruleset, *rolling, plan),
		)
	} else if plan != nil {
		ev.Outcomes = append(ev.Outcomes, evalLegumes(ruleset, nutrition.RollingSummary{}, plan))
	}

	ev.Score, ev.Coverage = aggregateScore(ruleset, ev.Outcomes)
	return ev
}

func evalFiber(ruleset Ruleset, sex eater.Sex, day nutrition.DaySummary) RuleOutcome {
	target := ruleset.FiberMinG[sex]
	cov := ruleCoverage(day.RuleCoverage, RuleFiber)
	if cov < ruleset.CoverageThreshold {
		return unknownOutcome(RuleFiber, SeveritySoft, &day.Totals.FiberG, &target, "g/day", cov)
	}
	actual := day.Totals.FiberG
	if actual >= target {
		return passOutcome(RuleFiber, SeveritySoft, &actual, &target, "g/day", cov)
	}
	return softFailOutcome(RuleFiber, SeveritySoft, &actual, &target, "g/day", cov)
}

func evalSodium(ruleset Ruleset, day nutrition.DaySummary) RuleOutcome {
	target := ruleset.SodiumMaxDailyMg
	cov := ruleCoverage(day.RuleCoverage, RuleSodium)
	if cov < ruleset.CoverageThreshold {
		return unknownOutcome(RuleSodium, SeveritySoft, &day.Totals.SodiumMg, &target, "mg/day", cov)
	}
	actual := day.Totals.SodiumMg
	if actual < target {
		return passOutcome(RuleSodium, SeveritySoft, &actual, &target, "mg/day", cov)
	}
	return softFailOutcome(RuleSodium, SeveritySoft, &actual, &target, "mg/day", cov)
}

func evalAddedSugar(ruleset Ruleset, day nutrition.DaySummary) RuleOutcome {
	targetMax := ruleset.AddedSugarMaxPctKcal
	targetWarn := ruleset.AddedSugarWarnPctKcal
	cov := ruleCoverage(day.RuleCoverage, RuleAddedSugar)

	if day.Macros.Kcal <= 0 || cov < ruleset.CoverageThreshold {
		return unknownOutcome(RuleAddedSugar, SeveritySoft, nil, &targetMax, "% kcal", cov)
	}

	actual := day.Totals.AddedSugarsG * kcalPerGramSugar / float64(day.Macros.Kcal) * scoreScalePercent
	if actual <= targetWarn {
		return passOutcome(RuleAddedSugar, SeveritySoft, &actual, &targetWarn, "% kcal", cov)
	}
	if actual < targetMax {
		return warnOutcome(RuleAddedSugar, SeveritySoft, &actual, &targetMax, "% kcal", cov)
	}
	return softFailOutcome(RuleAddedSugar, SeveritySoft, &actual, &targetMax, "% kcal", cov)
}

func evalSatFat(ruleset Ruleset, day nutrition.DaySummary) RuleOutcome {
	target := ruleset.SatFatMaxPctKcal
	cov := ruleCoverage(day.RuleCoverage, RuleSatFat)

	if day.Macros.Kcal <= 0 || cov < ruleset.CoverageThreshold {
		return unknownOutcome(RuleSatFat, SeveritySoft, nil, &target, "% kcal", cov)
	}

	actual := day.Totals.SaturatedFatG * kcalPerGramFat / float64(day.Macros.Kcal) * scoreScalePercent
	if actual < target {
		return passOutcome(RuleSatFat, SeveritySoft, &actual, &target, "% kcal", cov)
	}
	return softFailOutcome(RuleSatFat, SeveritySoft, &actual, &target, "% kcal", cov)
}

func evalIndustrialTransFat(ruleset Ruleset, day nutrition.DaySummary) RuleOutcome {
	target := 0.0
	cov := ruleCoverage(day.RuleCoverage, RuleIndustrialTransFat)
	if cov < ruleset.CoverageThreshold {
		return unknownOutcome(RuleIndustrialTransFat, SeverityWarn, &day.Totals.TransFatG, &target, "g/day", cov)
	}
	actual := day.Totals.TransFatG
	if actual > 0 {
		return warnOutcome(RuleIndustrialTransFat, SeverityWarn, &actual, &target, "g/day", cov)
	}
	return passOutcome(RuleIndustrialTransFat, SeverityWarn, &actual, &target, "g/day", cov)
}

func evalFruitVeg(ruleset Ruleset, day nutrition.DaySummary) RuleOutcome {
	target := ruleset.FruitVegMinG
	cov := ruleCoverage(day.RuleCoverage, RuleFruitVeg)
	if cov < ruleset.CoverageThreshold {
		return unknownOutcome(RuleFruitVeg, SeveritySoft, &day.Totals.FruitVegG, &target, "g/day", cov)
	}
	actual := day.Totals.FruitVegG
	if actual >= target {
		return passOutcome(RuleFruitVeg, SeveritySoft, &actual, &target, "g/day", cov)
	}
	return softFailOutcome(RuleFruitVeg, SeveritySoft, &actual, &target, "g/day", cov)
}

func evalWholeGrain(ruleset Ruleset, day nutrition.DaySummary) RuleOutcome {
	target := ruleset.WholeGrainRatioMin
	cov := ruleCoverage(day.RuleCoverage, RuleWholeGrain)
	if cov < ruleset.CoverageThreshold {
		return unknownOutcome(RuleWholeGrain, SeveritySoft, nil, &target, "ratio", cov)
	}

	var actual float64
	if day.Totals.GrainGEquiv == 0 {
		actual = 1.0
	} else {
		actual = day.Totals.WholeGrainGEquiv / day.Totals.GrainGEquiv
	}

	if actual >= target {
		return passOutcome(RuleWholeGrain, SeveritySoft, &actual, &target, "ratio", cov)
	}
	return softFailOutcome(RuleWholeGrain, SeveritySoft, &actual, &target, "ratio", cov)
}

func evalUPF(ruleset Ruleset, day nutrition.DaySummary) RuleOutcome {
	target := 0.0
	cov := ruleCoverage(day.RuleCoverage, RuleUPF)
	if cov < ruleset.CoverageThreshold {
		return unknownOutcome(RuleUPF, SeverityAnalytics, nil, &target, "share", cov)
	}

	share := day.Shares.UPFKcalShare(day.Macros.Kcal)
	if share == nil {
		return unknownOutcome(RuleUPF, SeverityAnalytics, nil, &target, "share", cov)
	}
	actual := *share
	if actual == 0 {
		return passOutcome(RuleUPF, SeverityAnalytics, &actual, &target, "share", cov)
	}
	score := math.Max(0, math.Min(1, 1-actual))
	return outcome(RuleUPF, nutrition.StatusWarn, SeverityAnalytics, &actual, &target, "share", cov, score)
}

func evalRedMeat(ruleset Ruleset, rolling nutrition.RollingSummary) RuleOutcome {
	target := ruleset.RedMeatMaxWeeklyG
	cov := ruleCoverage(rolling.RuleCoverage, RuleRedMeat)
	if cov < ruleset.CoverageThreshold {
		return unknownOutcome(RuleRedMeat, SeveritySoft, &rolling.RedMeatG, &target, "g/week", cov)
	}
	actual := rolling.RedMeatG
	if actual <= target {
		return passOutcome(RuleRedMeat, SeveritySoft, &actual, &target, "g/week", cov)
	}
	return softFailOutcome(RuleRedMeat, SeveritySoft, &actual, &target, "g/week", cov)
}

func evalProcessedMeat(ruleset Ruleset, rolling nutrition.RollingSummary) RuleOutcome {
	target := 0.0
	cov := ruleCoverage(rolling.RuleCoverage, RuleProcessedMeat)
	if cov < ruleset.CoverageThreshold {
		return unknownOutcome(RuleProcessedMeat, SeverityWarn, &rolling.ProcessedMeatG, &target, "g/week", cov)
	}
	actual := rolling.ProcessedMeatG
	if actual == 0 {
		return passOutcome(RuleProcessedMeat, SeverityWarn, &actual, &target, "g/week", cov)
	}
	if rolling.GeneratedProcessedMeatG > 0 {
		return softFailOutcome(RuleProcessedMeat, SeverityWarn, &actual, &target, "g/week", cov)
	}
	return warnOutcome(RuleProcessedMeat, SeverityWarn, &actual, &target, "g/week", cov)
}

func evalLegumes(ruleset Ruleset, rolling nutrition.RollingSummary, plan *PlanInput) RuleOutcome {
	target := legumeServingsTarget
	cov := ruleCoverage(rolling.RuleCoverage, RuleLegumes)
	actual := rolling.LegumeServings

	if plan != nil {
		componentTarget := 1.0
		componentActual := float64(plan.DistinctLegumeComponents)
		if componentActual >= componentTarget {
			return passOutcome(RuleLegumes, SeveritySoft, &componentActual, &componentTarget, "components/plan", 1.0)
		}
		if rolling.WindowDays == 0 {
			return softFailOutcome(RuleLegumes, SeveritySoft, &componentActual, &componentTarget, "components/plan", 1.0)
		}
	}

	if cov < ruleset.CoverageThreshold {
		return unknownOutcome(RuleLegumes, SeveritySoft, &actual, &target, "servings/week", cov)
	}
	if actual >= target {
		return passOutcome(RuleLegumes, SeveritySoft, &actual, &target, "servings/week", cov)
	}
	return softFailOutcome(RuleLegumes, SeveritySoft, &actual, &target, "servings/week", cov)
}

func ruleCoverage(coverage map[string]nutrition.Coverage, rule string) float64 {
	cov, ok := coverage[rule]
	if !ok {
		return 0
	}
	return cov.Score()
}

func aggregateScore(ruleset Ruleset, outcomes []RuleOutcome) (score, coverage float64) {
	var weightedScore, weightedCoverage, totalWeight float64
	for _, o := range outcomes {
		weight := ruleset.RuleWeights[o.Code]
		weightedScore += weight * o.Score
		weightedCoverage += weight * o.Coverage
		totalWeight += weight
	}
	if totalWeight == 0 {
		return 0, 0
	}
	return weightedScore / totalWeight * scoreScalePercent, weightedCoverage / totalWeight
}

func passOutcome(code string, severity RuleSeverity, actual, target *float64, unit string, coverage float64) RuleOutcome {
	return outcome(code, nutrition.StatusPass, severity, actual, target, unit, coverage, 1.0)
}

func warnOutcome(code string, severity RuleSeverity, actual, target *float64, unit string, coverage float64) RuleOutcome {
	return outcome(code, nutrition.StatusWarn, severity, actual, target, unit, coverage, warnScore)
}

func softFailOutcome(code string, severity RuleSeverity, actual, target *float64, unit string, coverage float64) RuleOutcome {
	return outcome(code, nutrition.StatusSoftFail, severity, actual, target, unit, coverage, 0.0)
}

func unknownOutcome(code string, severity RuleSeverity, actual, target *float64, unit string, coverage float64) RuleOutcome {
	return outcome(code, nutrition.StatusUnknown, severity, actual, target, unit, coverage, 0.0)
}

func outcome(code string, status nutrition.RuleEvalStatus, severity RuleSeverity, actual, target *float64, unit string, coverage, score float64) RuleOutcome {
	return RuleOutcome{
		Code:     code,
		Status:   status,
		Severity: severity,
		Actual:   actual,
		Target:   target,
		Unit:     unit,
		Coverage: coverage,
		Score:    score,
		Message:  explainOutcome(code, status, actual, target, unit),
	}
}
