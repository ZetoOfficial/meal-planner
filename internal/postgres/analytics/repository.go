// Package analytics reads the stored nutrition snapshots and recipe usage.
package analytics

import (
	"context"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
	apperr "github.com/ZetoOfficial/meal-planner/internal/errors"
	"github.com/ZetoOfficial/meal-planner/internal/postgres"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/dberror"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/pgvalue"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/sqlc"
	"github.com/ZetoOfficial/meal-planner/internal/usecase/reality"
)

type Repository struct {
	store *postgres.Store
}

func New(store *postgres.Store) *Repository {
	return &Repository{store: store}
}

func (r *Repository) ListDaySummaries(
	ctx context.Context,
	eaterID eaterdom.ID,
	basis nutrition.SummaryBasis,
	from, to time.Time,
) ([]nutrition.DaySummary, error) {
	rows, err := r.store.Queries(ctx).ListEaterDaySummariesForPeriod(ctx, sqlc.ListEaterDaySummariesForPeriodParams{
		EaterID:  int64(eaterID),
		Basis:    sqlc.SummaryBasis(basis),
		FromDate: pgvalue.Date(from),
		ToDate:   pgvalue.Date(to),
	})
	if err != nil {
		return nil, dberror.Map(err, "day nutrition summary")
	}
	summaries := make([]nutrition.DaySummary, 0, len(rows))
	for i := range rows {
		summary, err := daySummaryFromRow(rows[i])
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}
	return summaries, nil
}

func (r *Repository) ListRollingSummaries(
	ctx context.Context,
	eaterID eaterdom.ID,
	basis nutrition.SummaryBasis,
	from, to time.Time,
) ([]nutrition.RollingSummary, error) {
	rows, err := r.store.Queries(ctx).ListEaterRollingSummariesForPeriod(ctx, sqlc.ListEaterRollingSummariesForPeriodParams{
		EaterID:  int64(eaterID),
		Basis:    sqlc.SummaryBasis(basis),
		FromDate: pgvalue.Date(from),
		ToDate:   pgvalue.Date(to),
	})
	if err != nil {
		return nil, dberror.Map(err, "rolling nutrition summary")
	}
	summaries := make([]nutrition.RollingSummary, 0, len(rows))
	for i := range rows {
		summary, err := rollingSummaryFromRow(rows[i])
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}
	return summaries, nil
}

// ListRuleEvaluations reads the stored verdicts a period covers, both the daily
// ones and the rolling-window ones, so analytics can explain a score instead of
// only reporting it.
func (r *Repository) ListRuleEvaluations(
	ctx context.Context,
	eaterID eaterdom.ID,
	basis nutrition.SummaryBasis,
	from, to time.Time,
) ([]reality.RuleEvaluation, error) {
	id := int64(eaterID)
	rows, err := r.store.Queries(ctx).ListEaterRuleEvaluationsForPeriod(ctx, sqlc.ListEaterRuleEvaluationsForPeriodParams{
		EaterID:  &id,
		Basis:    sqlc.SummaryBasis(basis),
		FromDate: pgvalue.Date(from),
		ToDate:   pgvalue.Date(to),
	})
	if err != nil {
		return nil, dberror.Map(err, "rule evaluation result")
	}
	evaluations := make([]reality.RuleEvaluation, 0, len(rows))
	for i := range rows {
		evaluation, err := ruleEvaluationFromRow(rows[i])
		if err != nil {
			return nil, err
		}
		evaluations = append(evaluations, evaluation)
	}
	return evaluations, nil
}

func ruleEvaluationFromRow(row sqlc.RuleEvaluationResult) (reality.RuleEvaluation, error) {
	values, err := pgvalue.NumericFields(map[string]pgtype.Numeric{"coverage": row.CoverageScore})
	if err != nil {
		return reality.RuleEvaluation{}, err
	}
	actual, err := pgvalue.OptionalFloat64(row.ActualValue)
	if err != nil {
		return reality.RuleEvaluation{}, err
	}
	target, err := pgvalue.OptionalFloat64(row.TargetValue)
	if err != nil {
		return reality.RuleEvaluation{}, err
	}
	return reality.RuleEvaluation{
		Scope:       reality.RuleScope(row.EntityType),
		Date:        row.EvalDate.Time,
		WindowStart: row.WindowStart.Time,
		WindowEnd:   row.WindowEnd.Time,
		Result: nutrition.RuleEvaluationResult{
			RuleCode: row.RuleCode,
			Status:   nutrition.RuleEvalStatus(row.Status),
			Actual:   actual,
			Target:   target,
			Unit:     optionalText(row.Unit),
			Coverage: values["coverage"],
			Message:  optionalText(row.Message),
		},
	}, nil
}

func optionalText(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// ListTopRecipes ranks by how often a template was actually cooked, then by how
// often plans reached for it, so a new favourite is not hidden behind history.
func (r *Repository) ListTopRecipes(ctx context.Context, limit int) ([]reality.TopRecipe, error) {
	if limit <= 0 || int64(limit) > math.MaxInt32 {
		return nil, apperr.Validation("limit must fit a positive int32", "limit")
	}
	rows, err := r.store.Queries(ctx).ListTopRecipes(ctx, int32(limit))
	if err != nil {
		return nil, dberror.Map(err, "recipe")
	}
	recipes := make([]reality.TopRecipe, 0, len(rows))
	for i := range rows {
		recipes = append(recipes, reality.TopRecipe{
			RecipeID:    recipedom.ID(rows[i].ID),
			Name:        rows[i].Name,
			CookedCount: int(rows[i].CookedCount),
			PlanUsage:   int(rows[i].PlanUsage),
		})
	}
	return recipes, nil
}

func daySummaryFromRow(row sqlc.DayNutritionSummary) (nutrition.DaySummary, error) {
	values, err := pgvalue.NumericFields(map[string]pgtype.Numeric{
		"protein": row.ProteinG, "fat": row.FatG, "carb": row.CarbG,
		"satFat": row.SaturatedFatG, "transFat": row.TransFatG, "fiber": row.FiberG,
		"sodium": row.SodiumMg, "addedSugars": row.AddedSugarsG, "freeSugars": row.FreeSugarsG,
		"fruitVeg": row.FruitVegG, "grain": row.GrainGEquiv, "wholeGrain": row.WholeGrainGEquiv,
		"legume": row.LegumeG, "redMeat": row.RedMeatG, "processed": row.ProcessedMeatG,
		"coverage": row.DataCoverageScore,
	})
	if err != nil {
		return nutrition.DaySummary{}, err
	}
	coverage, err := nutrition.CoverageFromScore(values["coverage"])
	if err != nil {
		return nutrition.DaySummary{}, err
	}
	kcal := 0
	if row.Kcal != nil {
		kcal = int(*row.Kcal)
	}
	return nutrition.DaySummary{
		Basis:  nutrition.SummaryBasis(row.Basis),
		Macros: nutrition.Macros{ProteinG: values["protein"], FatG: values["fat"], CarbG: values["carb"], Kcal: kcal},
		Totals: nutrition.DailyTotals{
			SaturatedFatG: values["satFat"], TransFatG: values["transFat"], FiberG: values["fiber"],
			SodiumMg: values["sodium"], AddedSugarsG: values["addedSugars"], FreeSugarsG: values["freeSugars"],
			FruitVegG: values["fruitVeg"], GrainGEquiv: values["grain"], WholeGrainGEquiv: values["wholeGrain"],
			LegumeG: values["legume"], RedMeatG: values["redMeat"], ProcessedMeatG: values["processed"],
		},
		Coverage:     coverage,
		RuleCoverage: map[string]nutrition.Coverage{},
	}, nil
}

func rollingSummaryFromRow(row sqlc.RollingNutritionSummary) (nutrition.RollingSummary, error) {
	values, err := pgvalue.NumericFields(map[string]pgtype.Numeric{
		"redMeat": row.RedMeatG, "processed": row.ProcessedMeatG, "legume": row.LegumeServings,
		"fiber": row.FiberGAvg, "sodium": row.SodiumMgAvg, "satFat": row.SaturatedFatGAvg,
		"addedSugars": row.AddedSugarsGAvg, "fruitVeg": row.FruitVegGAvg,
		"coverage": row.DataCoverageScore,
	})
	if err != nil {
		return nutrition.RollingSummary{}, err
	}
	coverage, err := nutrition.CoverageFromScore(values["coverage"])
	if err != nil {
		return nutrition.RollingSummary{}, err
	}
	return nutrition.RollingSummary{
		Basis: nutrition.SummaryBasis(row.Basis), WindowDays: int(row.WindowDays),
		RedMeatG: values["redMeat"], ProcessedMeatG: values["processed"], LegumeServings: values["legume"],
		FiberGAvg: values["fiber"], SodiumMgAvg: values["sodium"], SaturatedFatGAvg: values["satFat"],
		AddedSugarsGAvg: values["addedSugars"], FruitVegGAvg: values["fruitVeg"],
		Coverage: coverage, RuleCoverage: map[string]nutrition.Coverage{},
	}, nil
}
