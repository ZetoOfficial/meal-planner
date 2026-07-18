package eater

import (
	"math"

	"github.com/jackc/pgx/v5/pgtype"

	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/pgvalue"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/sqlc"
)

func macroGoalParams(goal eaterdom.MacroGoal) sqlc.CreateMacroGoalParams {
	kcalMin := goal.Range.KcalMin
	kcalMax := goal.Range.KcalMax
	if kcalMin < 0 || int64(kcalMin) > math.MaxInt32 || kcalMax < 0 || int64(kcalMax) > math.MaxInt32 {
		panic("macro goal kcal must be a nonnegative int32")
	}
	return sqlc.CreateMacroGoalParams{
		EaterID: int64(goal.EaterID), EffectiveFrom: pgtype.Date{Time: goal.EffectiveFrom, Valid: true},
		ProteinMinG: pgvalue.Numeric(goal.Range.ProteinMinG), ProteinMaxG: pgvalue.Numeric(goal.Range.ProteinMaxG),
		FatMinG: pgvalue.Numeric(goal.Range.FatMinG), FatMaxG: pgvalue.Numeric(goal.Range.FatMaxG),
		CarbMinG: pgvalue.Numeric(goal.Range.CarbMinG), CarbMaxG: pgvalue.Numeric(goal.Range.CarbMaxG),
		KcalMin: int32(kcalMin), KcalMax: int32(kcalMax), Note: goal.Note,
	}
}

func macroGoalFromRow(row sqlc.EaterMacroGoal) (eaterdom.MacroGoal, error) {
	proteinMin, err := pgvalue.Float64(row.ProteinMinG)
	if err != nil {
		return eaterdom.MacroGoal{}, err
	}
	proteinMax, err := pgvalue.Float64(row.ProteinMaxG)
	if err != nil {
		return eaterdom.MacroGoal{}, err
	}
	fatMin, err := pgvalue.Float64(row.FatMinG)
	if err != nil {
		return eaterdom.MacroGoal{}, err
	}
	fatMax, err := pgvalue.Float64(row.FatMaxG)
	if err != nil {
		return eaterdom.MacroGoal{}, err
	}
	carbMin, err := pgvalue.Float64(row.CarbMinG)
	if err != nil {
		return eaterdom.MacroGoal{}, err
	}
	carbMax, err := pgvalue.Float64(row.CarbMaxG)
	if err != nil {
		return eaterdom.MacroGoal{}, err
	}
	return eaterdom.MacroGoal{ID: eaterdom.MacroGoalID(row.ID), EaterID: eaterdom.ID(row.EaterID), EffectiveFrom: row.EffectiveFrom.Time, Range: eaterdom.MacroRange{
		ProteinMinG: proteinMin, ProteinMaxG: proteinMax, FatMinG: fatMin, FatMaxG: fatMax, CarbMinG: carbMin, CarbMaxG: carbMax, KcalMin: int(row.KcalMin), KcalMax: int(row.KcalMax),
	}, Note: row.Note}, nil
}

func fixedMealParams(meal eaterdom.FixedMeal) sqlc.CreateFixedMealParams {
	kcal := meal.Macros.Kcal
	if kcal < 0 || int64(kcal) > math.MaxInt32 {
		panic("fixed meal kcal must be a nonnegative int32")
	}
	return sqlc.CreateFixedMealParams{
		EaterID:            int64(meal.EaterID),
		MealType:           sqlc.MealType(meal.MealType),
		Label:              meal.Label,
		ProteinG:           pgvalue.Numeric(meal.Macros.ProteinG),
		FatG:               pgvalue.Numeric(meal.Macros.FatG),
		CarbG:              pgvalue.Numeric(meal.Macros.CarbG),
		Kcal:               int32(kcal),
		SaturatedFatG:      pgvalue.OptionalNumeric(meal.Quality.SaturatedFatG),
		TransFatG:          pgvalue.OptionalNumeric(meal.Quality.TransFatG),
		FiberG:             pgvalue.OptionalNumeric(meal.Quality.FiberG),
		SodiumMg:           pgvalue.OptionalNumeric(meal.Quality.SodiumMg),
		AddedSugarsG:       pgvalue.OptionalNumeric(meal.Quality.AddedSugarsG),
		FreeSugarsG:        pgvalue.OptionalNumeric(meal.Quality.FreeSugarsG),
		ProcessedMeatG:     pgvalue.OptionalNumeric(meal.Quality.ProcessedMeatG),
		RedMeatG:           pgvalue.OptionalNumeric(meal.Quality.RedMeatG),
		WholeGrainGEquiv:   pgvalue.OptionalNumeric(meal.Quality.WholeGrainGEquiv),
		FruitVegG:          pgvalue.OptionalNumeric(meal.Quality.FruitVegG),
		IncludeInAnalytics: meal.Quality.IncludeInAnalytics,
		IncludeInWarnings:  meal.Quality.IncludeInWarnings,
		Active:             meal.Active,
	}
}

func fixedMealFromRow(row sqlc.FixedMeal) (eaterdom.FixedMeal, error) {
	protein, err := pgvalue.Float64(row.ProteinG)
	if err != nil {
		return eaterdom.FixedMeal{}, err
	}
	fat, err := pgvalue.Float64(row.FatG)
	if err != nil {
		return eaterdom.FixedMeal{}, err
	}
	carb, err := pgvalue.Float64(row.CarbG)
	if err != nil {
		return eaterdom.FixedMeal{}, err
	}
	quality, err := fixedMealQualityFromRow(row)
	if err != nil {
		return eaterdom.FixedMeal{}, err
	}
	return eaterdom.FixedMeal{ID: eaterdom.FixedMealID(row.ID), EaterID: eaterdom.ID(row.EaterID), MealType: eaterdom.MealType(row.MealType), Label: row.Label, Macros: eaterdom.Macros{ProteinG: protein, FatG: fat, CarbG: carb, Kcal: int(row.Kcal)}, Quality: quality, Active: row.Active}, nil
}

func fixedMealQualityFromRow(row sqlc.FixedMeal) (eaterdom.FixedMealQuality, error) {
	saturated, err := pgvalue.OptionalFloat64(row.SaturatedFatG)
	if err != nil {
		return eaterdom.FixedMealQuality{}, err
	}
	trans, err := pgvalue.OptionalFloat64(row.TransFatG)
	if err != nil {
		return eaterdom.FixedMealQuality{}, err
	}
	fiber, err := pgvalue.OptionalFloat64(row.FiberG)
	if err != nil {
		return eaterdom.FixedMealQuality{}, err
	}
	sodium, err := pgvalue.OptionalFloat64(row.SodiumMg)
	if err != nil {
		return eaterdom.FixedMealQuality{}, err
	}
	added, err := pgvalue.OptionalFloat64(row.AddedSugarsG)
	if err != nil {
		return eaterdom.FixedMealQuality{}, err
	}
	free, err := pgvalue.OptionalFloat64(row.FreeSugarsG)
	if err != nil {
		return eaterdom.FixedMealQuality{}, err
	}
	processed, err := pgvalue.OptionalFloat64(row.ProcessedMeatG)
	if err != nil {
		return eaterdom.FixedMealQuality{}, err
	}
	red, err := pgvalue.OptionalFloat64(row.RedMeatG)
	if err != nil {
		return eaterdom.FixedMealQuality{}, err
	}
	wholeGrain, err := pgvalue.OptionalFloat64(row.WholeGrainGEquiv)
	if err != nil {
		return eaterdom.FixedMealQuality{}, err
	}
	fruitVeg, err := pgvalue.OptionalFloat64(row.FruitVegG)
	if err != nil {
		return eaterdom.FixedMealQuality{}, err
	}
	return eaterdom.FixedMealQuality{SaturatedFatG: saturated, TransFatG: trans, FiberG: fiber, SodiumMg: sodium, AddedSugarsG: added, FreeSugarsG: free, ProcessedMeatG: processed, RedMeatG: red, WholeGrainGEquiv: wholeGrain, FruitVegG: fruitVeg, IncludeInAnalytics: row.IncludeInAnalytics, IncludeInWarnings: row.IncludeInWarnings}, nil
}

func exclusionFromRow(row sqlc.EaterExclusion) eaterdom.Exclusion {
	return eaterdom.Exclusion{ID: eaterdom.ExclusionID(row.ID), EaterID: eaterdom.ID(row.EaterID), TargetKind: eaterdom.ExclusionTarget(row.TargetKind), TargetValue: row.TargetValue, Severity: eaterdom.ExclusionSeverity(row.Severity), Note: row.Note}
}
