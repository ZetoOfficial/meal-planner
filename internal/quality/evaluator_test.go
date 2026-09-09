package quality

import (
	"math"
	"testing"

	"github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
)

func fullCoverage(rules ...string) map[string]nutrition.Coverage {
	m := make(map[string]nutrition.Coverage)
	for _, r := range rules {
		c, _ := nutrition.CoverageForInput(true, 1)
		m[r] = c
	}
	return m
}

func TestEvaluateFiberPassAndFail(t *testing.T) {
	rs := DefaultRuleset()

	pass := nutrition.DaySummary{
		Macros:       nutrition.Macros{Kcal: 1800},
		RuleCoverage: fullCoverage(RuleFiber),
		Totals:       nutrition.DailyTotals{FiberG: 30},
	}
	ev := Evaluate(rs, eater.SexFemale, pass, nil, nil)
	outcome := findOutcome(ev, RuleFiber)
	if outcome.Status != nutrition.StatusPass {
		t.Fatalf("fiber pass status = %s, want pass", outcome.Status)
	}

	fail := nutrition.DaySummary{
		Macros:       nutrition.Macros{Kcal: 1800},
		RuleCoverage: fullCoverage(RuleFiber),
		Totals:       nutrition.DailyTotals{FiberG: 15},
	}
	ev = Evaluate(rs, eater.SexFemale, fail, nil, nil)
	outcome = findOutcome(ev, RuleFiber)
	if outcome.Status != nutrition.StatusSoftFail {
		t.Fatalf("fiber fail status = %s, want soft_fail", outcome.Status)
	}
}

func TestEvaluateFiberSexSpecific(t *testing.T) {
	rs := DefaultRuleset()
	day := nutrition.DaySummary{
		Macros:       nutrition.Macros{Kcal: 1800},
		RuleCoverage: fullCoverage(RuleFiber),
		Totals:       nutrition.DailyTotals{FiberG: 30},
	}

	if got := Evaluate(rs, eater.SexFemale, day, nil, nil); findOutcome(got, RuleFiber).Status != nutrition.StatusPass {
		t.Fatal("female fiber target should pass at 30 g")
	}
	if got := Evaluate(rs, eater.SexMale, day, nil, nil); findOutcome(got, RuleFiber).Status != nutrition.StatusSoftFail {
		t.Fatal("male fiber target should fail at 30 g")
	}
}

func TestEvaluateSodium(t *testing.T) {
	rs := DefaultRuleset()

	pass := nutrition.DaySummary{
		RuleCoverage: fullCoverage(RuleSodium),
		Totals:       nutrition.DailyTotals{SodiumMg: 2000},
	}
	if got := findOutcome(Evaluate(rs, eater.SexFemale, pass, nil, nil), RuleSodium); got.Status != nutrition.StatusPass {
		t.Fatalf("sodium pass status = %s", got.Status)
	}

	fail := nutrition.DaySummary{
		RuleCoverage: fullCoverage(RuleSodium),
		Totals:       nutrition.DailyTotals{SodiumMg: 2500},
	}
	if got := findOutcome(Evaluate(rs, eater.SexFemale, fail, nil, nil), RuleSodium); got.Status != nutrition.StatusSoftFail {
		t.Fatalf("sodium fail status = %s", got.Status)
	}
}

func TestEvaluateAddedSugar(t *testing.T) {
	rs := DefaultRuleset()
	base := nutrition.DaySummary{
		Macros:       nutrition.Macros{Kcal: 2000},
		RuleCoverage: fullCoverage(RuleAddedSugar),
	}

	pass := base
	pass.Totals.AddedSugarsG = 5 // 5*4/2000*100 = 1%
	if got := findOutcome(Evaluate(rs, eater.SexFemale, pass, nil, nil), RuleAddedSugar); got.Status != nutrition.StatusPass {
		t.Fatalf("added sugar pass status = %s", got.Status)
	}

	warn := base
	warn.Totals.AddedSugarsG = 30 // 6%
	if got := findOutcome(Evaluate(rs, eater.SexFemale, warn, nil, nil), RuleAddedSugar); got.Status != nutrition.StatusWarn {
		t.Fatalf("added sugar warn status = %s", got.Status)
	}

	fail := base
	fail.Totals.AddedSugarsG = 60 // 12%
	if got := findOutcome(Evaluate(rs, eater.SexFemale, fail, nil, nil), RuleAddedSugar); got.Status != nutrition.StatusSoftFail {
		t.Fatalf("added sugar fail status = %s", got.Status)
	}

	unknown := base
	unknown.RuleCoverage = map[string]nutrition.Coverage{}
	if got := findOutcome(Evaluate(rs, eater.SexFemale, unknown, nil, nil), RuleAddedSugar); got.Status != nutrition.StatusUnknown {
		t.Fatalf("added sugar unknown status = %s", got.Status)
	}
}

func TestEvaluateSatFat(t *testing.T) {
	rs := DefaultRuleset()
	base := nutrition.DaySummary{
		Macros:       nutrition.Macros{Kcal: 2000},
		RuleCoverage: fullCoverage(RuleSatFat),
	}

	pass := base
	pass.Totals.SaturatedFatG = 20 // 20*9/2000*100 = 9%
	if got := findOutcome(Evaluate(rs, eater.SexFemale, pass, nil, nil), RuleSatFat); got.Status != nutrition.StatusPass {
		t.Fatalf("sat fat pass status = %s", got.Status)
	}

	fail := base
	fail.Totals.SaturatedFatG = 25 // 11.25%
	if got := findOutcome(Evaluate(rs, eater.SexFemale, fail, nil, nil), RuleSatFat); got.Status != nutrition.StatusSoftFail {
		t.Fatalf("sat fat fail status = %s", got.Status)
	}
}

func TestEvaluateIndustrialTransFat(t *testing.T) {
	rs := DefaultRuleset()
	base := nutrition.DaySummary{
		RuleCoverage: fullCoverage(RuleIndustrialTransFat),
	}

	pass := base
	pass.Totals.TransFatG = 0
	if got := findOutcome(Evaluate(rs, eater.SexFemale, pass, nil, nil), RuleIndustrialTransFat); got.Status != nutrition.StatusPass {
		t.Fatalf("trans fat pass status = %s", got.Status)
	}

	warn := base
	warn.Totals.TransFatG = 1
	if got := findOutcome(Evaluate(rs, eater.SexFemale, warn, nil, nil), RuleIndustrialTransFat); got.Status != nutrition.StatusWarn {
		t.Fatalf("trans fat warn status = %s", got.Status)
	}
}

func TestEvaluateFruitVeg(t *testing.T) {
	rs := DefaultRuleset()
	base := nutrition.DaySummary{RuleCoverage: fullCoverage(RuleFruitVeg)}

	pass := base
	pass.Totals.FruitVegG = 500
	if got := findOutcome(Evaluate(rs, eater.SexFemale, pass, nil, nil), RuleFruitVeg); got.Status != nutrition.StatusPass {
		t.Fatalf("fruit/veg pass status = %s", got.Status)
	}

	fail := base
	fail.Totals.FruitVegG = 200
	if got := findOutcome(Evaluate(rs, eater.SexFemale, fail, nil, nil), RuleFruitVeg); got.Status != nutrition.StatusSoftFail {
		t.Fatalf("fruit/veg fail status = %s", got.Status)
	}
}

func TestEvaluateWholeGrainZeroDenominator(t *testing.T) {
	rs := DefaultRuleset()
	day := nutrition.DaySummary{
		RuleCoverage: fullCoverage(RuleWholeGrain),
		Totals:       nutrition.DailyTotals{GrainGEquiv: 0},
	}
	got := findOutcome(Evaluate(rs, eater.SexFemale, day, nil, nil), RuleWholeGrain)
	if got.Status != nutrition.StatusPass {
		t.Fatalf("whole grain zero denominator status = %s, want pass", got.Status)
	}
	if got.Actual == nil || *got.Actual != 1.0 {
		t.Fatalf("whole grain ratio = %v, want 1.0", got.Actual)
	}
}

func TestEvaluateWholeGrainRatio(t *testing.T) {
	rs := DefaultRuleset()
	day := nutrition.DaySummary{
		RuleCoverage: fullCoverage(RuleWholeGrain),
		Totals:       nutrition.DailyTotals{GrainGEquiv: 100, WholeGrainGEquiv: 40},
	}
	got := findOutcome(Evaluate(rs, eater.SexFemale, day, nil, nil), RuleWholeGrain)
	if got.Status != nutrition.StatusSoftFail {
		t.Fatalf("whole grain 0.4 ratio status = %s, want soft_fail", got.Status)
	}
}

func TestEvaluateUPF(t *testing.T) {
	rs := DefaultRuleset()
	zero := 0.0
	pass := nutrition.DaySummary{
		Macros:       nutrition.Macros{Kcal: 2000},
		RuleCoverage: fullCoverage(RuleUPF),
		Shares:       nutrition.DailyShares{UPFKcal: &zero},
	}
	if got := findOutcome(Evaluate(rs, eater.SexFemale, pass, nil, nil), RuleUPF); got.Status != nutrition.StatusPass {
		t.Fatalf("upf pass status = %s", got.Status)
	}

	warn := nutrition.DaySummary{
		Macros:       nutrition.Macros{Kcal: 2000},
		RuleCoverage: fullCoverage(RuleUPF),
		Shares:       nutrition.DailyShares{UPFKcal: float64Ptr(400)},
	}
	if got := findOutcome(Evaluate(rs, eater.SexFemale, warn, nil, nil), RuleUPF); got.Status != nutrition.StatusWarn || math.Abs(got.Score-0.8) > 1e-9 {
		t.Fatalf("upf outcome = status %s score %v, want warn and 0.8", got.Status, got.Score)
	}
}

func TestEvaluateRedMeatRolling(t *testing.T) {
	rs := DefaultRuleset()
	rolling := nutrition.RollingSummary{
		WindowDays:   7,
		RuleCoverage: fullCoverage(RuleRedMeat),
		RedMeatG:     300,
	}
	if got := findOutcome(Evaluate(rs, eater.SexFemale, nutrition.DaySummary{}, &rolling, nil), RuleRedMeat); got.Status != nutrition.StatusPass {
		t.Fatalf("red meat pass status = %s", got.Status)
	}

	rolling.RedMeatG = 400
	if got := findOutcome(Evaluate(rs, eater.SexFemale, nutrition.DaySummary{}, &rolling, nil), RuleRedMeat); got.Status != nutrition.StatusSoftFail {
		t.Fatalf("red meat fail status = %s", got.Status)
	}
}

func TestEvaluateProcessedMeatRolling(t *testing.T) {
	rs := DefaultRuleset()

	pass := nutrition.RollingSummary{
		WindowDays:   7,
		RuleCoverage: fullCoverage(RuleProcessedMeat),
	}
	if got := findOutcome(Evaluate(rs, eater.SexFemale, nutrition.DaySummary{}, &pass, nil), RuleProcessedMeat); got.Status != nutrition.StatusPass {
		t.Fatalf("processed meat pass status = %s", got.Status)
	}

	warn := nutrition.RollingSummary{
		WindowDays:          7,
		RuleCoverage:        fullCoverage(RuleProcessedMeat),
		ProcessedMeatG:      100,
		FixedProcessedMeatG: 100,
	}
	if got := findOutcome(Evaluate(rs, eater.SexFemale, nutrition.DaySummary{}, &warn, nil), RuleProcessedMeat); got.Status != nutrition.StatusWarn {
		t.Fatalf("processed meat warn status = %s", got.Status)
	}

	fail := nutrition.RollingSummary{
		WindowDays:              7,
		RuleCoverage:            fullCoverage(RuleProcessedMeat),
		ProcessedMeatG:          100,
		GeneratedProcessedMeatG: 100,
	}
	if got := findOutcome(Evaluate(rs, eater.SexFemale, nutrition.DaySummary{}, &fail, nil), RuleProcessedMeat); got.Status != nutrition.StatusSoftFail {
		t.Fatalf("processed meat fail status = %s", got.Status)
	}
}

func TestEvaluateLegumes(t *testing.T) {
	rs := DefaultRuleset()
	rolling := nutrition.RollingSummary{
		WindowDays:     7,
		RuleCoverage:   fullCoverage(RuleLegumes),
		LegumeServings: 2,
	}

	if got := findOutcome(Evaluate(rs, eater.SexFemale, nutrition.DaySummary{}, &rolling, &PlanInput{DistinctLegumeComponents: 1}), RuleLegumes); got.Status != nutrition.StatusPass {
		t.Fatalf("legumes pass by component status = %s", got.Status)
	}

	rolling.LegumeServings = 4
	if got := findOutcome(Evaluate(rs, eater.SexFemale, nutrition.DaySummary{}, &rolling, nil), RuleLegumes); got.Status != nutrition.StatusPass {
		t.Fatalf("legumes pass by servings status = %s", got.Status)
	}

	rolling.LegumeServings = 1
	if got := findOutcome(Evaluate(rs, eater.SexFemale, nutrition.DaySummary{}, &rolling, nil), RuleLegumes); got.Status != nutrition.StatusSoftFail {
		t.Fatalf("legumes fail status = %s", got.Status)
	}
}

func TestEvaluateLegumesPlanWithoutRollingHistory(t *testing.T) {
	rs := DefaultRuleset()

	pass := findOutcome(Evaluate(rs, eater.SexFemale, nutrition.DaySummary{}, nil, &PlanInput{DistinctLegumeComponents: 1}), RuleLegumes)
	if pass.Status != nutrition.StatusPass || pass.Unit != "components/plan" {
		t.Fatalf("plan legumes pass = status %s unit %s", pass.Status, pass.Unit)
	}

	fail := findOutcome(Evaluate(rs, eater.SexFemale, nutrition.DaySummary{}, nil, &PlanInput{}), RuleLegumes)
	if fail.Status != nutrition.StatusSoftFail {
		t.Fatalf("plan legumes fail status = %s, want soft_fail", fail.Status)
	}
}

func TestMacroPassQualityFailIsWarningNotBlock(t *testing.T) {
	rs := DefaultRuleset()
	zero := 0.0
	day := nutrition.DaySummary{
		Macros:       nutrition.Macros{Kcal: 2000},
		RuleCoverage: fullCoverage(RuleFiber, RuleSodium, RuleAddedSugar, RuleSatFat, RuleIndustrialTransFat, RuleFruitVeg, RuleWholeGrain, RuleUPF),
		Totals:       nutrition.DailyTotals{FiberG: 10, SodiumMg: 5000, SaturatedFatG: 30, AddedSugarsG: 80, FruitVegG: 100, GrainGEquiv: 100, WholeGrainGEquiv: 10},
		Shares:       nutrition.DailyShares{UPFKcal: &zero},
	}
	ev := Evaluate(rs, eater.SexFemale, day, nil, nil)

	var hasFail bool
	for _, o := range ev.Outcomes {
		if o.Status == nutrition.StatusSoftFail || o.Status == nutrition.StatusWarn {
			hasFail = true
		}
		if o.Status == nutrition.StatusUnknown {
			t.Fatal("no rule should be unknown with full coverage")
		}
	}
	if !hasFail {
		t.Fatal("expected at least one quality failure/warning")
	}
	if ev.Score == 100 {
		t.Fatal("quality score should be below 100 when rules fail")
	}
}

func findOutcome(ev Evaluation, code string) RuleOutcome {
	for _, o := range ev.Outcomes {
		if o.Code == code {
			return o
		}
	}
	panic("outcome not found: " + code)
}

func TestAggregateOutcomesWeighsOnlyTheSuppliedRules(t *testing.T) {
	ruleset := DefaultRuleset()
	outcomes := []RuleOutcome{
		passOutcome(RuleFiber, SeveritySoft, nil, nil, "g/day", 1),
		softFailOutcome(RuleSodium, SeveritySoft, nil, nil, "mg/day", 0.5),
	}

	score, coverage := AggregateOutcomes(ruleset, outcomes)

	if score != 50 {
		t.Fatalf("score = %v, want 50 for one pass and one soft fail", score)
	}
	if math.Abs(coverage-0.75) > 1e-9 {
		t.Fatalf("coverage = %v, want the mean of the supplied rule coverages", coverage)
	}
}

func TestAggregateOutcomesReturnsZeroWithoutWeightedRules(t *testing.T) {
	score, coverage := AggregateOutcomes(DefaultRuleset(), nil)

	if score != 0 || coverage != 0 {
		t.Fatalf("AggregateOutcomes(nil) = (%v, %v), want zero values", score, coverage)
	}
}
