package quality

import (
	"testing"

	"github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
)

func TestBuildRollingSummarySevenDays(t *testing.T) {
	rs := DefaultRuleset()
	days := make([]*nutrition.DaySummary, 7)
	for i := range days {
		days[i] = &nutrition.DaySummary{
			Macros:       nutrition.Macros{Kcal: 2000},
			RuleCoverage: fullCoverage(RuleRedMeat, RuleProcessedMeat, RuleLegumes),
			Totals: nutrition.DailyTotals{
				RedMeatG:                40,
				ProcessedMeatG:          10,
				GeneratedProcessedMeatG: 10,
				LegumeG:                 150,
			},
		}
	}

	rolling, err := BuildRollingSummary(rs, 7, days)
	if err != nil {
		t.Fatalf("BuildRollingSummary() error = %v", err)
	}

	if rolling.RedMeatG != 280 {
		t.Fatalf("red meat = %v, want 280", rolling.RedMeatG)
	}
	if rolling.ProcessedMeatG != 70 {
		t.Fatalf("processed meat = %v, want 70", rolling.ProcessedMeatG)
	}
	if rolling.LegumeServings != 7.0 {
		t.Fatalf("legume servings = %v, want 7.0", rolling.LegumeServings)
	}
}

func TestBuildRollingSummaryRejectsDaysOutsideWindow(t *testing.T) {
	days := make([]*nutrition.DaySummary, 8)
	if _, err := BuildRollingSummary(DefaultRuleset(), 7, days); err == nil {
		t.Fatal("BuildRollingSummary() with eight days in a seven-day window should error")
	}
}

func TestBuildRollingSummaryMissingHistoryIsUnknown(t *testing.T) {
	rs := DefaultRuleset()
	days := []*nutrition.DaySummary{
		{
			Macros:       nutrition.Macros{Kcal: 2000},
			RuleCoverage: fullCoverage(RuleRedMeat),
			Totals:       nutrition.DailyTotals{RedMeatG: 100},
		},
		{
			Macros:       nutrition.Macros{Kcal: 2000},
			RuleCoverage: fullCoverage(RuleRedMeat),
			Totals:       nutrition.DailyTotals{RedMeatG: 100},
		},
		{
			Macros:       nutrition.Macros{Kcal: 2000},
			RuleCoverage: fullCoverage(RuleRedMeat),
			Totals:       nutrition.DailyTotals{RedMeatG: 100},
		},
		nil, nil, nil, nil,
	}

	rolling, err := BuildRollingSummary(rs, 7, days)
	if err != nil {
		t.Fatalf("BuildRollingSummary() error = %v", err)
	}

	if rolling.RedMeatG != 300 {
		t.Fatalf("red meat = %v, want 300 (no zero imputation)", rolling.RedMeatG)
	}

	cov := rolling.RuleCoverage[RuleRedMeat].Score()
	if cov >= 1.0 {
		t.Fatalf("red meat coverage = %v, should be below 1 with missing history", cov)
	}

	ev := Evaluate(rs, eater.SexFemale, nutrition.DaySummary{}, &rolling, nil)
	if got := findOutcome(ev, RuleRedMeat); got.Status != nutrition.StatusUnknown {
		t.Fatalf("red meat status = %s, want unknown due to missing history", got.Status)
	}
}

func TestBuildRollingSummaryProcessedMeatFixedVsGenerated(t *testing.T) {
	rs := DefaultRuleset()
	days := make([]*nutrition.DaySummary, 7)
	for i := range days {
		days[i] = &nutrition.DaySummary{
			Macros:       nutrition.Macros{Kcal: 2000},
			RuleCoverage: fullCoverage(RuleProcessedMeat),
			Totals:       nutrition.DailyTotals{ProcessedMeatG: 50, FixedProcessedMeatG: 50},
		}
	}

	rolling, err := BuildRollingSummary(rs, 7, days)
	if err != nil {
		t.Fatalf("BuildRollingSummary() error = %v", err)
	}
	if rolling.GeneratedProcessedMeatG != 0 {
		t.Fatalf("generated processed meat = %v, want 0", rolling.GeneratedProcessedMeatG)
	}
	if rolling.FixedProcessedMeatG == 0 {
		t.Fatal("fixed processed meat should be positive")
	}

	ev := Evaluate(rs, eater.SexFemale, nutrition.DaySummary{}, &rolling, nil)
	if got := findOutcome(ev, RuleProcessedMeat); got.Status != nutrition.StatusWarn {
		t.Fatalf("processed meat status = %s, want warn for fixed-only", got.Status)
	}
}
