package quality

import (
	"errors"

	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
)

func BuildRollingSummary(ruleset Ruleset, windowDays int, days []*nutrition.DaySummary) (nutrition.RollingSummary, error) {
	if windowDays <= 0 {
		return nutrition.RollingSummary{}, errors.New("window days must be positive")
	}
	if len(days) > windowDays {
		return nutrition.RollingSummary{}, errors.New("day summaries exceed rolling window")
	}
	if ruleset.LegumeServingSizeG <= 0 {
		return nutrition.RollingSummary{}, errors.New("legume serving size must be positive")
	}

	result := nutrition.RollingSummary{
		Basis:        nutrition.SummaryBasisPlanned,
		WindowDays:   windowDays,
		RuleCoverage: make(map[string]nutrition.Coverage),
	}

	var basis nutrition.SummaryBasis
	knownDays := 0
	for _, day := range days {
		if day == nil {
			continue
		}
		if basis == "" {
			basis = day.Basis
		} else if day.Basis != "" && day.Basis != basis {
			return nutrition.RollingSummary{}, errors.New("rolling summary cannot mix planned and actual day bases")
		}
		knownDays++
		addDayToRolling(&result, *day)
	}
	if basis != "" {
		result.Basis = basis
	}

	if knownDays == 0 {
		return result, nil
	}

	result.FiberGAvg /= float64(knownDays)
	result.SodiumMgAvg /= float64(knownDays)
	result.SaturatedFatGAvg /= float64(knownDays)
	result.AddedSugarsGAvg /= float64(knownDays)
	result.FruitVegGAvg /= float64(knownDays)

	result.LegumeServings /= ruleset.LegumeServingSizeG

	scaleCoverageForMissingDays(&result, knownDays, windowDays)
	result.Coverage = aggregateCoverage(result.RuleCoverage)
	return result, nil
}

func addDayToRolling(rolling *nutrition.RollingSummary, day nutrition.DaySummary) {
	rolling.RedMeatG += day.Totals.RedMeatG
	rolling.ProcessedMeatG += day.Totals.ProcessedMeatG
	rolling.GeneratedProcessedMeatG += day.Totals.GeneratedProcessedMeatG
	rolling.FixedProcessedMeatG += day.Totals.FixedProcessedMeatG

	rolling.FiberGAvg += day.Totals.FiberG
	rolling.SodiumMgAvg += day.Totals.SodiumMg
	rolling.SaturatedFatGAvg += day.Totals.SaturatedFatG
	rolling.AddedSugarsGAvg += day.Totals.AddedSugarsG
	rolling.FruitVegGAvg += day.Totals.FruitVegG

	rolling.LegumeServings += day.Totals.LegumeG

	for rule, cov := range day.RuleCoverage {
		rolling.RuleCoverage[rule] = rolling.RuleCoverage[rule].Add(cov)
	}
}

func scaleCoverageForMissingDays(rolling *nutrition.RollingSummary, knownDays, windowDays int) {
	if windowDays == 0 || knownDays >= windowDays {
		return
	}
	factor := float64(knownDays) / float64(windowDays)
	for rule, cov := range rolling.RuleCoverage {
		scaled := cov.Score() * factor
		known, _ := nutrition.CoverageForInput(true, scaled)
		unknown, _ := nutrition.CoverageForInput(false, 1-scaled)
		rolling.RuleCoverage[rule] = known.Add(unknown)
	}
}
