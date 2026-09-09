package planning

import (
	"math"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/pgvalue"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/sqlc"
)

func daySummaryParams(
	planID planningdom.PlanID,
	snapshot planningdom.QualitySnapshot,
) sqlc.UpsertDayNutritionSummaryParams {
	totals := snapshot.Day.Totals
	kcal := snapshot.Day.Macros.Kcal
	if kcal < 0 || int64(kcal) > math.MaxInt32 {
		panic("day summary kcal must be a nonnegative int32")
	}
	return sqlc.UpsertDayNutritionSummaryParams{
		PlanID:            int64(planID),
		EaterID:           int64(snapshot.EaterID),
		SummaryDate:       pgvalue.Date(snapshot.Date),
		Basis:             sqlc.SummaryBasis(snapshot.BasisOrPlanned()),
		ProteinG:          pgvalue.Numeric(snapshot.Day.Macros.ProteinG),
		FatG:              pgvalue.Numeric(snapshot.Day.Macros.FatG),
		CarbG:             pgvalue.Numeric(snapshot.Day.Macros.CarbG),
		Kcal:              int32Pointer(int32(kcal)),
		SaturatedFatG:     pgvalue.Numeric(totals.SaturatedFatG),
		TransFatG:         pgvalue.Numeric(totals.TransFatG),
		FiberG:            pgvalue.Numeric(totals.FiberG),
		SodiumMg:          pgvalue.Numeric(totals.SodiumMg),
		AddedSugarsG:      pgvalue.Numeric(totals.AddedSugarsG),
		FreeSugarsG:       pgvalue.Numeric(totals.FreeSugarsG),
		FruitVegG:         pgvalue.Numeric(totals.FruitVegG),
		GrainGEquiv:       pgvalue.Numeric(totals.GrainGEquiv),
		WholeGrainGEquiv:  pgvalue.Numeric(totals.WholeGrainGEquiv),
		LegumeG:           pgvalue.Numeric(totals.LegumeG),
		RedMeatG:          pgvalue.Numeric(totals.RedMeatG),
		ProcessedMeatG:    pgvalue.Numeric(totals.ProcessedMeatG),
		UpfKcalShareEst:   pgvalue.OptionalNumeric(snapshot.Day.Shares.UPFKcalShare(kcal)),
		DataCoverageScore: pgvalue.Numeric(snapshot.Day.Coverage.Score()),
	}
}

func rollingSummaryParams(
	planID planningdom.PlanID,
	snapshot planningdom.QualitySnapshot,
) sqlc.UpsertRollingNutritionSummaryParams {
	rolling := snapshot.Rolling
	if rolling.WindowDays <= 0 || rolling.WindowDays > math.MaxInt16 {
		panic("rolling window days must be a positive int16")
	}
	return sqlc.UpsertRollingNutritionSummaryParams{
		PlanID:            int64(planID),
		EaterID:           int64(snapshot.EaterID),
		AsOfDate:          pgvalue.Date(snapshot.Date),
		WindowDays:        int16(rolling.WindowDays),
		Basis:             sqlc.SummaryBasis(snapshot.BasisOrPlanned()),
		RedMeatG:          pgvalue.Numeric(rolling.RedMeatG),
		ProcessedMeatG:    pgvalue.Numeric(rolling.ProcessedMeatG),
		LegumeServings:    pgvalue.Numeric(rolling.LegumeServings),
		FiberGAvg:         pgvalue.Numeric(rolling.FiberGAvg),
		SodiumMgAvg:       pgvalue.Numeric(rolling.SodiumMgAvg),
		SaturatedFatGAvg:  pgvalue.Numeric(rolling.SaturatedFatGAvg),
		AddedSugarsGAvg:   pgvalue.Numeric(rolling.AddedSugarsGAvg),
		FruitVegGAvg:      pgvalue.Numeric(rolling.FruitVegGAvg),
		DataCoverageScore: pgvalue.Numeric(rolling.Coverage.Score()),
	}
}

func ruleResultParams(
	entityType planningdom.RuleEntityType,
	entityID int64,
	snapshot planningdom.QualitySnapshot,
	result nutrition.RuleEvaluationResult,
) sqlc.UpsertRuleEvaluationResultParams {
	eaterID := int64(snapshot.EaterID)
	params := sqlc.UpsertRuleEvaluationResultParams{
		EntityType:    sqlc.RuleEntityType(entityType),
		EntityID:      entityID,
		EaterID:       &eaterID,
		RuleCode:      result.RuleCode,
		Basis:         sqlc.SummaryBasis(snapshot.BasisOrPlanned()),
		Status:        sqlc.RuleEvalStatus(result.Status),
		ActualValue:   pgvalue.OptionalNumeric(result.Actual),
		TargetValue:   pgvalue.OptionalNumeric(result.Target),
		Unit:          optionalText(result.Unit),
		CoverageScore: pgvalue.Numeric(result.Coverage),
		Message:       optionalText(result.Message),
	}
	if entityType == planningdom.RuleEntityRollingWindow {
		params.WindowStart = pgvalue.Date(snapshot.WindowStart())
		params.WindowEnd = pgvalue.Date(snapshot.Date)
		return params
	}
	params.EvalDate = pgvalue.Date(snapshot.Date)
	return params
}

func daySummaryFromRow(row sqlc.DayNutritionSummary) (planningdom.QualitySnapshot, error) {
	values, err := pgvalue.NumericFields(map[string]pgtype.Numeric{
		"protein":      row.ProteinG,
		"fat":          row.FatG,
		"carb":         row.CarbG,
		"satFat":       row.SaturatedFatG,
		"transFat":     row.TransFatG,
		"fiber":        row.FiberG,
		"sodium":       row.SodiumMg,
		"addedSugars":  row.AddedSugarsG,
		"freeSugars":   row.FreeSugarsG,
		"fruitVeg":     row.FruitVegG,
		"grain":        row.GrainGEquiv,
		"wholeGrain":   row.WholeGrainGEquiv,
		"legume":       row.LegumeG,
		"redMeat":      row.RedMeatG,
		"processed":    row.ProcessedMeatG,
		"coverageOnly": row.DataCoverageScore,
	})
	if err != nil {
		return planningdom.QualitySnapshot{}, err
	}
	coverage, err := nutrition.CoverageFromScore(values["coverageOnly"])
	if err != nil {
		return planningdom.QualitySnapshot{}, err
	}

	kcal := 0
	if row.Kcal != nil {
		kcal = int(*row.Kcal)
	}
	return planningdom.QualitySnapshot{
		EaterID: eaterdom.ID(row.EaterID),
		Date:    row.SummaryDate.Time,
		Basis:   nutrition.SummaryBasis(row.Basis),
		Day: nutrition.DaySummary{
			Basis:  nutrition.SummaryBasis(row.Basis),
			Macros: nutrition.Macros{ProteinG: values["protein"], FatG: values["fat"], CarbG: values["carb"], Kcal: kcal},
			Totals: nutrition.DailyTotals{
				SaturatedFatG:    values["satFat"],
				TransFatG:        values["transFat"],
				FiberG:           values["fiber"],
				SodiumMg:         values["sodium"],
				AddedSugarsG:     values["addedSugars"],
				FreeSugarsG:      values["freeSugars"],
				FruitVegG:        values["fruitVeg"],
				GrainGEquiv:      values["grain"],
				WholeGrainGEquiv: values["wholeGrain"],
				LegumeG:          values["legume"],
				RedMeatG:         values["redMeat"],
				ProcessedMeatG:   values["processed"],
			},
			Coverage:     coverage,
			RuleCoverage: map[string]nutrition.Coverage{},
		},
	}, nil
}

func rollingSummaryFromRow(row sqlc.RollingNutritionSummary) (nutrition.RollingSummary, error) {
	values, err := pgvalue.NumericFields(map[string]pgtype.Numeric{
		"redMeat":      row.RedMeatG,
		"processed":    row.ProcessedMeatG,
		"legume":       row.LegumeServings,
		"fiber":        row.FiberGAvg,
		"sodium":       row.SodiumMgAvg,
		"satFat":       row.SaturatedFatGAvg,
		"addedSugars":  row.AddedSugarsGAvg,
		"fruitVeg":     row.FruitVegGAvg,
		"coverageOnly": row.DataCoverageScore,
	})
	if err != nil {
		return nutrition.RollingSummary{}, err
	}
	coverage, err := nutrition.CoverageFromScore(values["coverageOnly"])
	if err != nil {
		return nutrition.RollingSummary{}, err
	}
	return nutrition.RollingSummary{
		Basis:            nutrition.SummaryBasis(row.Basis),
		WindowDays:       int(row.WindowDays),
		RedMeatG:         values["redMeat"],
		ProcessedMeatG:   values["processed"],
		LegumeServings:   values["legume"],
		FiberGAvg:        values["fiber"],
		SodiumMgAvg:      values["sodium"],
		SaturatedFatGAvg: values["satFat"],
		AddedSugarsGAvg:  values["addedSugars"],
		FruitVegGAvg:     values["fruitVeg"],
		Coverage:         coverage,
		RuleCoverage:     map[string]nutrition.Coverage{},
	}, nil
}

func ruleResultFromRow(row sqlc.RuleEvaluationResult) (nutrition.RuleEvaluationResult, error) {
	actual, err := pgvalue.OptionalFloat64(row.ActualValue)
	if err != nil {
		return nutrition.RuleEvaluationResult{}, err
	}
	target, err := pgvalue.OptionalFloat64(row.TargetValue)
	if err != nil {
		return nutrition.RuleEvaluationResult{}, err
	}
	coverage, err := pgvalue.Float64(row.CoverageScore)
	if err != nil {
		return nutrition.RuleEvaluationResult{}, err
	}
	return nutrition.RuleEvaluationResult{
		RuleCode: row.RuleCode,
		Status:   nutrition.RuleEvalStatus(row.Status),
		Actual:   actual,
		Target:   target,
		Unit:     textValue(row.Unit),
		Coverage: coverage,
		Message:  textValue(row.Message),
	}, nil
}

// coverageFromScore rebuilds an aggregate coverage from its stored score; the
// per-rule weights behind it are not persisted because only the score is used.

func optionalText(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func textValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func int32Pointer(value int32) *int32 { return &value }

// snapshotKey addresses one eater-day snapshot while stitching day summaries,
// rolling projections and rule verdicts back together.
func snapshotKey(eaterID eaterdom.ID, date time.Time) string {
	return strconv.FormatInt(int64(eaterID), 10) + "@" + date.UTC().Format(time.DateOnly)
}
