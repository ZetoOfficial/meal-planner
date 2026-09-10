package reality

import (
	"context"
	"time"

	"github.com/ZetoOfficial/meal-planner/internal/apperr"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	"github.com/ZetoOfficial/meal-planner/internal/quality"
	"github.com/ZetoOfficial/meal-planner/internal/usecase/reference"
)

const dayDuration = 24 * time.Hour

type Period struct {
	EaterID eaterdom.ID
	From    time.Time
	To      time.Time
}

func (p Period) validate() error {
	if !p.EaterID.Valid() {
		return apperr.Validation("eaterId must be positive", "eaterId")
	}
	if p.From.IsZero() || p.To.IsZero() {
		return apperr.Validation("from and to are required", "from")
	}
	if p.To.Before(p.From) {
		return apperr.Validation("to must not be before from", "to")
	}
	return nil
}

func (p Period) normalized() Period {
	p.From = p.From.UTC().Truncate(dayDuration)
	p.To = p.To.UTC().Truncate(dayDuration)
	return p
}

// MacroAnalytics averages a period per day. Planned uses the solver's numbers;
// actual uses the logged ones and falls back to planned where nothing was
// logged, which is what a NULL actual means (FR-6).
type MacroAnalytics struct {
	Period      Period
	Days        int
	AvgPlanned  nutrition.Macros
	AvgActual   nutrition.Macros
	HasActual   bool
	SkippedDays int
}

func (s *Service) MacroAnalytics(ctx context.Context, period Period) (MacroAnalytics, error) {
	if err := period.validate(); err != nil {
		return MacroAnalytics{}, err
	}
	period = period.normalized()

	meals, err := s.meals.ListEaterMeals(ctx, period.EaterID, period.From, period.To)
	if err != nil {
		return MacroAnalytics{}, err
	}

	planned := make(map[string]nutrition.Macros)
	actual := make(map[string]nutrition.Macros)
	for i := range meals {
		meal := &meals[i]
		key := meal.Date.UTC().Format(time.DateOnly)
		if planned[key], err = planned[key].Add(meal.Planned); err != nil {
			return MacroAnalytics{}, err
		}
		if !meal.Eaten() {
			continue
		}
		if actual[key], err = actual[key].Add(meal.EffectiveMacros()); err != nil {
			return MacroAnalytics{}, err
		}
	}

	result := MacroAnalytics{Period: period, Days: len(planned)}
	if result.Days == 0 {
		return result, nil
	}
	if result.AvgPlanned, err = averageMacros(planned, result.Days); err != nil {
		return MacroAnalytics{}, err
	}
	if result.AvgActual, err = averageMacros(actual, result.Days); err != nil {
		return MacroAnalytics{}, err
	}
	result.HasActual = len(actual) > 0
	return result, nil
}

func averageMacros(byDay map[string]nutrition.Macros, days int) (nutrition.Macros, error) {
	total := nutrition.Macros{}
	for _, macros := range byDay {
		sum, err := total.Add(macros)
		if err != nil {
			return nutrition.Macros{}, err
		}
		total = sum
	}
	return total.Scale(1 / float64(days))
}

// QualityAnalytics reports the stored hot-path averages, rule verdicts and
// coverage for a period.
type QualityAnalytics struct {
	Period      Period
	Basis       nutrition.SummaryBasis
	Days        int
	Averages    nutrition.DailyTotals
	Coverage    float64
	Rolling     nutrition.RollingSummary
	Evaluations []RuleEvaluation
}

// RuleScope says whether a stored verdict covers one day or a rolling window.
// The two are read back together but describe different periods.
type RuleScope string

const (
	RuleScopeDay           RuleScope = "day"
	RuleScopeRollingWindow RuleScope = "rolling_window"
)

// RuleEvaluation is one persisted verdict together with the period it covers.
type RuleEvaluation struct {
	Scope       RuleScope
	Date        time.Time
	WindowStart time.Time
	WindowEnd   time.Time
	Result      nutrition.RuleEvaluationResult
}

func (s *Service) QualityAnalytics(
	ctx context.Context,
	period Period,
	basis nutrition.SummaryBasis,
) (QualityAnalytics, error) {
	if err := period.validate(); err != nil {
		return QualityAnalytics{}, err
	}
	period = period.normalized()
	if basis != nutrition.SummaryBasisPlanned && basis != nutrition.SummaryBasisActual {
		return QualityAnalytics{}, apperr.Validation("basis must be planned or actual", "basis")
	}

	days, err := s.analytics.ListDaySummaries(ctx, period.EaterID, basis, period.From, period.To)
	if err != nil {
		return QualityAnalytics{}, err
	}
	rolling, err := s.analytics.ListRollingSummaries(ctx, period.EaterID, basis, period.From, period.To)
	if err != nil {
		return QualityAnalytics{}, err
	}

	result := QualityAnalytics{Period: period, Basis: basis, Days: len(days)}
	if len(days) == 0 {
		return result, nil
	}
	evaluations, err := s.analytics.ListRuleEvaluations(ctx, period.EaterID, basis, period.From, period.To)
	if err != nil {
		return QualityAnalytics{}, err
	}

	result.Averages, result.Coverage = averageTotals(days)
	result.Evaluations = evaluations
	if len(rolling) > 0 {
		result.Rolling = rolling[len(rolling)-1]
	}
	return result, nil
}

func averageTotals(days []nutrition.DaySummary) (totals nutrition.DailyTotals, coverage float64) {
	for i := range days {
		day := &days[i]
		totals.FiberG += day.Totals.FiberG
		totals.SodiumMg += day.Totals.SodiumMg
		totals.SaturatedFatG += day.Totals.SaturatedFatG
		totals.AddedSugarsG += day.Totals.AddedSugarsG
		totals.FruitVegG += day.Totals.FruitVegG
		totals.GrainGEquiv += day.Totals.GrainGEquiv
		totals.WholeGrainGEquiv += day.Totals.WholeGrainGEquiv
		totals.LegumeG += day.Totals.LegumeG
		totals.RedMeatG += day.Totals.RedMeatG
		totals.ProcessedMeatG += day.Totals.ProcessedMeatG
		coverage += day.Coverage.Score()
	}
	count := float64(len(days))
	totals.FiberG /= count
	totals.SodiumMg /= count
	totals.SaturatedFatG /= count
	totals.AddedSugarsG /= count
	totals.FruitVegG /= count
	totals.GrainGEquiv /= count
	totals.WholeGrainGEquiv /= count
	totals.LegumeG /= count
	return totals, coverage / count
}

// refreshActualSummaries recomputes the actual-basis snapshot of one plan from
// the meal log. Snapshots are keyed by plan, eater, date and basis, so
// recomputing replaces rather than duplicates.
func (s *Service) refreshActualSummaries(ctx context.Context, planID planningdom.PlanID) error {
	plan, err := s.meals.Get(ctx, planID)
	if err != nil {
		return err
	}
	catalogue, err := reference.LoadCatalogue(ctx, s.reference, []planningdom.Plan{plan})
	if err != nil {
		return err
	}
	snapshots, err := actualSnapshots(plan, catalogue, s.ruleset(), s.windowDays())
	if err != nil {
		return err
	}
	return s.meals.SaveQualitySnapshots(ctx, planID, snapshots)
}

func actualSnapshots(
	plan planningdom.Plan,
	catalogue quality.Catalogue,
	ruleset quality.Ruleset,
	windowDays int,
) ([]planningdom.QualitySnapshot, error) {
	days, err := quality.BuildPlanDaysForBasis(plan, catalogue, nutrition.SummaryBasisActual)
	if err != nil {
		return nil, err
	}

	history := make(map[eaterdom.ID][]nutrition.DaySummary, len(days))
	snapshots := make([]planningdom.QualitySnapshot, 0, len(days))
	for i := range days {
		day := &days[i]
		summary, err := quality.BuildDaySummary(day.Input, quality.SummaryPurposeAnalytics)
		if err != nil {
			return nil, err
		}
		summary.Basis = nutrition.SummaryBasisActual

		window := history[day.EaterID]
		if len(window) > windowDays-1 {
			window = window[len(window)-(windowDays-1):]
		}
		pointers := make([]*nutrition.DaySummary, 0, len(window)+1)
		for j := range window {
			pointers = append(pointers, &window[j])
		}
		pointers = append(pointers, &summary)

		rolling, err := quality.BuildRollingSummary(ruleset, windowDays, pointers)
		if err != nil {
			return nil, err
		}
		rolling.Basis = nutrition.SummaryBasisActual
		evaluation := quality.Evaluate(ruleset, day.Input.Sex, summary, &rolling, nil)

		history[day.EaterID] = append(history[day.EaterID], summary)
		snapshots = append(snapshots, planningdom.QualitySnapshot{
			EaterID:      day.EaterID,
			Date:         day.Date,
			Basis:        nutrition.SummaryBasisActual,
			Day:          summary,
			Rolling:      rolling,
			DayRules:     splitRules(evaluation, false),
			RollingRules: splitRules(evaluation, true),
		})
	}
	return snapshots, nil
}

func splitRules(evaluation quality.Evaluation, rolling bool) []nutrition.RuleEvaluationResult {
	results := make([]nutrition.RuleEvaluationResult, 0, len(evaluation.Outcomes))
	for _, outcome := range evaluation.Outcomes {
		if quality.IsRollingRule(outcome.Code) != rolling {
			continue
		}
		results = append(results, outcome.Result())
	}
	return results
}

func (s *Service) TopRecipes(ctx context.Context, limit int) ([]TopRecipe, error) {
	if limit <= 0 {
		limit = defaultTopRecipes
	}
	return s.analytics.ListTopRecipes(ctx, limit)
}

const (
	defaultTopRecipes = 10
	defaultWindowDays = 7
)

func (s *Service) ruleset() quality.Ruleset { return quality.DefaultRuleset() }

func (s *Service) windowDays() int { return defaultWindowDays }
