package quality

import (
	"math"
	"testing"

	"github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	"github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
)

func float64Ptr(v float64) *float64 { return &v }
func boolPtr(v bool) *bool          { return &v }

func macrosPer100G(p, f, c, k float64) nutrition.MacrosPer100G {
	return nutrition.MacrosPer100G{ProteinG: p, FatG: f, CarbG: c, Kcal: k}
}

func TestBuildDaySummaryEmpty(t *testing.T) {
	input := DayInput{Sex: eater.SexFemale}
	summary, err := BuildDaySummary(input, SummaryPurposeAnalytics)
	if err != nil {
		t.Fatalf("BuildDaySummary() error = %v", err)
	}
	if summary.Macros.Kcal != 0 {
		t.Fatalf("empty summary kcal = %d, want 0", summary.Macros.Kcal)
	}
	if summary.Coverage.Score() != 0 {
		t.Fatalf("empty summary coverage = %v, want 0", summary.Coverage.Score())
	}
}

func TestBuildDaySummaryInvalidSex(t *testing.T) {
	_, err := BuildDaySummary(DayInput{Sex: eater.Sex("")}, SummaryPurposeAnalytics)
	if err == nil {
		t.Fatal("BuildDaySummary() with invalid sex should error")
	}
}

func TestBuildDaySummaryOneComponent(t *testing.T) {
	input := DayInput{
		Sex: eater.SexFemale,
		Meals: []MealInput{
			{
				Components: []ComponentInput{
					{
						PortionFactor: 0.5,
						Slots: []SlotInput{
							{
								PlannedInputQtyG:  200,
								PreparedQtyFactor: float64Ptr(0.9),
								MacrosPer100G:     macrosPer100G(20, 5, 0, 120),
								Facts: catalog.NutritionFacts{
									FiberPer100G:        float64Ptr(2),
									SodiumMgPer100G:     float64Ptr(50),
									SaturatedFatPer100G: float64Ptr(1),
									AddedSugarsPer100G:  float64Ptr(0),
								},
								Classification: catalog.FoodClassification{
									FoodGroup:        ptrFoodGroup(catalog.FoodGroupVegetable),
									CountsToFruitVeg: boolPtr(true),
									ProcessingLevel:  catalog.ProcessingLevelMinimallyProcessed,
								},
							},
						},
					},
				},
			},
		},
	}

	summary, err := BuildDaySummary(input, SummaryPurposeAnalytics)
	if err != nil {
		t.Fatalf("BuildDaySummary() error = %v", err)
	}

	if summary.Macros.Kcal != 120 {
		t.Fatalf("kcal = %d, want 120", summary.Macros.Kcal)
	}
	if summary.Totals.FiberG != 2.0 {
		t.Fatalf("fiber = %v, want 2", summary.Totals.FiberG)
	}
	if summary.Totals.SodiumMg != 50.0 {
		t.Fatalf("sodium = %v, want 50", summary.Totals.SodiumMg)
	}
	if summary.Totals.FruitVegG != 90.0 {
		t.Fatalf("fruit/veg = %v, want 90", summary.Totals.FruitVegG)
	}

	fiberCov := summary.RuleCoverage[RuleFiber].Score()
	if fiberCov != 1 {
		t.Fatalf("fiber coverage = %v, want 1", fiberCov)
	}
}

func TestBuildDaySummaryUnknownNutrientDoesNotCredit(t *testing.T) {
	input := DayInput{
		Sex: eater.SexFemale,
		Meals: []MealInput{
			{
				Components: []ComponentInput{
					{
						PortionFactor: 1,
						Slots: []SlotInput{
							{
								PlannedInputQtyG: 100,
								MacrosPer100G:    macrosPer100G(0, 0, 0, 0),
								Facts:            catalog.NutritionFacts{},
								Classification: catalog.FoodClassification{
									ProcessingLevel: catalog.ProcessingLevelUnknown,
								},
							},
						},
					},
				},
			},
		},
	}

	summary, err := BuildDaySummary(input, SummaryPurposeAnalytics)
	if err != nil {
		t.Fatalf("BuildDaySummary() error = %v", err)
	}
	if summary.Totals.FiberG != 0 {
		t.Fatalf("unknown fiber should not credit, got %v", summary.Totals.FiberG)
	}
	if summary.RuleCoverage[RuleFiber].Score() != 0 {
		t.Fatalf("unknown fiber coverage should be 0, got %v", summary.RuleCoverage[RuleFiber].Score())
	}
}

func TestBuildDaySummaryFixedMealFlags(t *testing.T) {
	input := DayInput{
		Sex: eater.SexFemale,
		Meals: []MealInput{
			{Fixed: &FixedMealInput{
				Macros: nutrition.Macros{ProteinG: 20, FatG: 10, CarbG: 30, Kcal: 400},
				Quality: eater.FixedMealQuality{
					SodiumMg:           float64Ptr(500),
					IncludeInAnalytics: false,
					IncludeInWarnings:  true,
				},
			}},
		},
	}

	analytics, err := BuildDaySummary(input, SummaryPurposeAnalytics)
	if err != nil {
		t.Fatalf("BuildDaySummary() error = %v", err)
	}
	if analytics.Macros.Kcal != 400 || analytics.Macros.ProteinG != 20 {
		t.Fatalf("analytics fixed meal macros = %+v, want kcal=400 protein=20", analytics.Macros)
	}
	if analytics.Totals.SodiumMg != 0 {
		t.Fatalf("analytics sodium = %v, want 0", analytics.Totals.SodiumMg)
	}

	warnings, err := BuildDaySummary(input, SummaryPurposeWarnings)
	if err != nil {
		t.Fatalf("BuildDaySummary() error = %v", err)
	}
	if warnings.Macros.Kcal != 400 || warnings.Macros.ProteinG != 20 {
		t.Fatalf("warnings fixed meal macros = %+v, want kcal=400 protein=20", warnings.Macros)
	}
	if warnings.Totals.SodiumMg != 500 {
		t.Fatalf("warnings sodium = %v, want 500", warnings.Totals.SodiumMg)
	}

	input.Meals[0].Fixed.Quality.IncludeInAnalytics = true
	input.Meals[0].Fixed.Quality.IncludeInWarnings = false
	analytics, err = BuildDaySummary(input, SummaryPurposeAnalytics)
	if err != nil {
		t.Fatalf("BuildDaySummary() error = %v", err)
	}
	warnings, err = BuildDaySummary(input, SummaryPurposeWarnings)
	if err != nil {
		t.Fatalf("BuildDaySummary() error = %v", err)
	}
	if analytics.Totals.SodiumMg != 500 || warnings.Totals.SodiumMg != 0 {
		t.Fatalf("independent fixed flags produced analytics=%v warnings=%v, want 500 and 0", analytics.Totals.SodiumMg, warnings.Totals.SodiumMg)
	}
	if warnings.Macros.Kcal != 400 {
		t.Fatalf("excluded warning quality changed fixed macros to %+v", warnings.Macros)
	}
}

func TestBuildDaySummaryUnknownClassificationReducesCoverage(t *testing.T) {
	prepared := 1.0
	notRedMeat := false
	input := DayInput{
		Sex: eater.SexFemale,
		Meals: []MealInput{{Components: []ComponentInput{{
			PortionFactor: 1,
			Slots: []SlotInput{
				{
					PlannedInputQtyG:  100,
					PreparedQtyFactor: &prepared,
					MacrosPer100G:     macrosPer100G(0, 0, 0, 0),
					Classification:    catalog.FoodClassification{IsRedMeat: &notRedMeat},
				},
				{
					PlannedInputQtyG:  200,
					PreparedQtyFactor: &prepared,
					MacrosPer100G:     macrosPer100G(0, 0, 0, 0),
				},
			},
		}}}},
	}

	summary, err := BuildDaySummary(input, SummaryPurposeAnalytics)
	if err != nil {
		t.Fatalf("BuildDaySummary() error = %v", err)
	}
	if got := summary.RuleCoverage[RuleRedMeat].Score(); math.Abs(got-1.0/3.0) > 1e-9 {
		t.Fatalf("red meat coverage = %v, want 1/3", got)
	}
	rolling, err := BuildRollingSummary(DefaultRuleset(), 1, []*nutrition.DaySummary{&summary})
	if err != nil {
		t.Fatalf("BuildRollingSummary() error = %v", err)
	}
	got := findOutcome(Evaluate(DefaultRuleset(), eater.SexFemale, nutrition.DaySummary{}, &rolling, nil), RuleRedMeat)
	if got.Status != nutrition.StatusUnknown {
		t.Fatalf("red meat status = %s, want unknown", got.Status)
	}
}

func TestBuildDaySummaryUPFShare(t *testing.T) {
	input := DayInput{
		Sex: eater.SexFemale,
		Meals: []MealInput{
			{
				Components: []ComponentInput{
					{
						PortionFactor: 1,
						Slots: []SlotInput{
							{
								PlannedInputQtyG: 100,
								MacrosPer100G:    macrosPer100G(0, 0, 50, 200),
								Facts:            catalog.NutritionFacts{},
								Classification: catalog.FoodClassification{
									ProcessingLevel: catalog.ProcessingLevelUltraProcessed,
								},
							},
						},
					},
				},
			},
		},
	}

	summary, err := BuildDaySummary(input, SummaryPurposeAnalytics)
	if err != nil {
		t.Fatalf("BuildDaySummary() error = %v", err)
	}
	if summary.Shares.UPFKcal == nil || *summary.Shares.UPFKcal != 200 {
		t.Fatalf("upf kcal = %v, want 200", summary.Shares.UPFKcal)
	}
	share := summary.Shares.UPFKcalShare(summary.Macros.Kcal)
	if share == nil || *share != 1.0 {
		t.Fatalf("upf share = %v, want 1.0", share)
	}
}

func TestDetectHardViolationsPHO(t *testing.T) {
	input := DayInput{
		Sex: eater.SexFemale,
		Meals: []MealInput{
			{
				Components: []ComponentInput{
					{
						PortionFactor: 1,
						Slots: []SlotInput{
							{
								PlannedInputQtyG: 100,
								MacrosPer100G:    macrosPer100G(0, 0, 0, 0),
								Facts: catalog.NutritionFacts{
									ContainsPartiallyHydrogenatedOils: boolPtr(true),
								},
							},
						},
					},
				},
			},
		},
	}

	violations := DetectHardViolations(input)
	if len(violations) != 1 {
		t.Fatalf("hard violations = %d, want 1", len(violations))
	}
	if violations[0].RuleCode != RuleIndustrialTransFat {
		t.Fatalf("rule code = %s, want %s", violations[0].RuleCode, RuleIndustrialTransFat)
	}
}

func TestDetectHardViolationsIndustrialAmount(t *testing.T) {
	input := DayInput{
		Sex: eater.SexFemale,
		Meals: []MealInput{
			{
				Components: []ComponentInput{
					{
						PortionFactor: 1,
						Slots: []SlotInput{
							{
								PlannedInputQtyG: 100,
								MacrosPer100G:    macrosPer100G(0, 0, 0, 0),
								Facts: catalog.NutritionFacts{
									IndustrialTransFatPer100G: float64Ptr(0.5),
								},
							},
						},
					},
				},
			},
		},
	}

	violations := DetectHardViolations(input)
	if len(violations) != 1 {
		t.Fatalf("hard violations = %d, want 1", len(violations))
	}
}

func ptrFoodGroup(g catalog.FoodGroup) *catalog.FoodGroup { return &g }
