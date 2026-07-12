package repositories

import (
	"context"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/sqlc"
)

type Eater struct {
	repositories *Repositories
}

func NewEater(repositories *Repositories) *Eater { return &Eater{repositories: repositories} }

func (r *Eater) Create(ctx context.Context, profile eaterdom.Eater, goal eaterdom.MacroGoal) (eaterdom.Eater, error) {
	query := r.repositories.Queries(ctx)
	row, err := query.CreateEater(ctx, sqlc.CreateEaterParams{Name: profile.Name, Sex: sqlc.Sex(profile.Sex)})
	if err != nil {
		return eaterdom.Eater{}, mapCatalogError(err, "eater")
	}
	profile.ID, goal.EaterID = eaterdom.ID(row.ID), eaterdom.ID(row.ID)
	if err := r.replaceAllergens(ctx, query, profile.ID, profile.Allergens); err != nil {
		return eaterdom.Eater{}, err
	}
	if _, err := r.createGoal(ctx, query, goal); err != nil {
		return eaterdom.Eater{}, err
	}
	return r.load(ctx, query, profile.ID)
}

func (r *Eater) List(ctx context.Context) ([]eaterdom.Eater, error) {
	query := r.repositories.Queries(ctx)
	rows, err := query.ListEaters(ctx)
	if err != nil {
		return nil, mapCatalogError(err, "eater")
	}
	profiles := make([]eaterdom.Eater, 0, len(rows))
	for _, row := range rows {
		profile, err := r.load(ctx, query, eaterdom.ID(row.ID))
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, profile)
	}
	return profiles, nil
}

func (r *Eater) Get(ctx context.Context, id eaterdom.ID) (eaterdom.Eater, error) {
	return r.load(ctx, r.repositories.Queries(ctx), id)
}

func (r *Eater) Update(ctx context.Context, profile eaterdom.Eater, goal eaterdom.MacroGoal) (eaterdom.Eater, error) {
	query := r.repositories.Queries(ctx)
	if _, err := query.UpdateEater(ctx, sqlc.UpdateEaterParams{ID: int64(profile.ID), Name: profile.Name, Sex: sqlc.Sex(profile.Sex)}); err != nil {
		return eaterdom.Eater{}, mapCatalogError(err, "eater")
	}
	if err := r.replaceAllergens(ctx, query, profile.ID, profile.Allergens); err != nil {
		return eaterdom.Eater{}, err
	}
	if _, err := r.createGoal(ctx, query, goal); err != nil {
		return eaterdom.Eater{}, err
	}
	return r.load(ctx, query, profile.ID)
}

func (r *Eater) ListGoals(ctx context.Context, eaterID eaterdom.ID) ([]eaterdom.MacroGoal, error) {
	query := r.repositories.Queries(ctx)
	if err := r.ensureEater(ctx, query, eaterID); err != nil {
		return nil, err
	}
	rows, err := query.ListMacroGoals(ctx, int64(eaterID))
	if err != nil {
		return nil, mapCatalogError(err, "macro goal")
	}
	goals := make([]eaterdom.MacroGoal, 0, len(rows))
	for i := range rows {
		goal, err := macroGoalFromRow(rows[i])
		if err != nil {
			return nil, err
		}
		goals = append(goals, goal)
	}
	return goals, nil
}

func (r *Eater) CreateGoal(ctx context.Context, goal eaterdom.MacroGoal) (eaterdom.MacroGoal, error) {
	query := r.repositories.Queries(ctx)
	if err := r.ensureEater(ctx, query, goal.EaterID); err != nil {
		return eaterdom.MacroGoal{}, err
	}
	return r.createGoal(ctx, query, goal)
}

func (r *Eater) ListFixedMeals(ctx context.Context, eaterID eaterdom.ID) ([]eaterdom.FixedMeal, error) {
	query := r.repositories.Queries(ctx)
	if err := r.ensureEater(ctx, query, eaterID); err != nil {
		return nil, err
	}
	rows, err := query.ListActiveFixedMeals(ctx, int64(eaterID))
	if err != nil {
		return nil, mapCatalogError(err, "fixed meal")
	}
	meals := make([]eaterdom.FixedMeal, 0, len(rows))
	for i := range rows {
		meal, err := fixedMealFromRow(rows[i])
		if err != nil {
			return nil, err
		}
		meals = append(meals, meal)
	}
	return meals, nil
}

func (r *Eater) CreateFixedMeal(ctx context.Context, meal eaterdom.FixedMeal) (eaterdom.FixedMeal, error) {
	query := r.repositories.Queries(ctx)
	if err := r.ensureEater(ctx, query, meal.EaterID); err != nil {
		return eaterdom.FixedMeal{}, err
	}
	row, err := query.CreateFixedMeal(ctx, fixedMealParams(meal))
	if err != nil {
		return eaterdom.FixedMeal{}, mapCatalogError(err, "fixed meal")
	}
	return fixedMealFromRow(row)
}

func (r *Eater) ListExclusions(ctx context.Context, eaterID eaterdom.ID) ([]eaterdom.Exclusion, error) {
	query := r.repositories.Queries(ctx)
	if err := r.ensureEater(ctx, query, eaterID); err != nil {
		return nil, err
	}
	rows, err := query.ListEaterExclusions(ctx, int64(eaterID))
	if err != nil {
		return nil, mapCatalogError(err, "exclusion")
	}
	exclusions := make([]eaterdom.Exclusion, 0, len(rows))
	for i := range rows {
		exclusions = append(exclusions, exclusionFromRow(rows[i]))
	}
	return exclusions, nil
}

func (r *Eater) CreateExclusion(ctx context.Context, exclusion eaterdom.Exclusion) (eaterdom.Exclusion, error) {
	query := r.repositories.Queries(ctx)
	if err := r.ensureEater(ctx, query, exclusion.EaterID); err != nil {
		return eaterdom.Exclusion{}, err
	}
	row, err := query.CreateEaterExclusion(ctx, sqlc.CreateEaterExclusionParams{
		EaterID: int64(exclusion.EaterID), TargetKind: sqlc.ExclusionTarget(exclusion.TargetKind), TargetValue: exclusion.TargetValue,
		Severity: sqlc.ExclusionSeverity(exclusion.Severity), Note: exclusion.Note,
	})
	if err != nil {
		return eaterdom.Exclusion{}, mapCatalogError(err, "exclusion")
	}
	return exclusionFromRow(row), nil
}

func (r *Eater) DeleteExclusion(ctx context.Context, id eaterdom.ExclusionID) error {
	if _, err := r.repositories.Queries(ctx).DeleteEaterExclusion(ctx, int64(id)); err != nil {
		return mapCatalogError(err, "exclusion")
	}
	return nil
}

func (r *Eater) RoleTagExists(ctx context.Context, code string) (bool, error) {
	exists, err := r.repositories.Queries(ctx).RoleTagExists(ctx, code)
	return exists, mapCatalogError(err, "role tag")
}

func (r *Eater) ProductExists(ctx context.Context, id int64) (bool, error) {
	exists, err := r.repositories.Queries(ctx).ProductExists(ctx, id)
	return exists, mapCatalogError(err, "product")
}

func (r *Eater) load(ctx context.Context, query *sqlc.Queries, id eaterdom.ID) (eaterdom.Eater, error) {
	row, err := query.GetEater(ctx, int64(id))
	if err != nil {
		return eaterdom.Eater{}, mapCatalogError(err, "eater")
	}
	profile := eaterdom.Eater{ID: eaterdom.ID(row.ID), Name: row.Name, Sex: eaterdom.Sex(row.Sex)}
	allergens, err := query.ListEaterAllergens(ctx, int64(id))
	if err != nil {
		return eaterdom.Eater{}, mapCatalogError(err, "eater allergens")
	}
	profile.Allergens = make([]eaterdom.Allergen, len(allergens))
	for i, allergen := range allergens {
		profile.Allergens[i] = eaterdom.Allergen(allergen)
	}
	exclusions, err := query.ListEaterExclusions(ctx, int64(id))
	if err != nil {
		return eaterdom.Eater{}, mapCatalogError(err, "exclusion")
	}
	profile.Exclusions = make([]eaterdom.Exclusion, 0, len(exclusions))
	for _, exclusion := range exclusions {
		profile.Exclusions = append(profile.Exclusions, exclusionFromRow(exclusion))
	}
	goals, err := query.ListMacroGoals(ctx, int64(id))
	if err != nil {
		return eaterdom.Eater{}, mapCatalogError(err, "macro goal")
	}
	domainGoals := make([]eaterdom.MacroGoal, 0, len(goals))
	for i := range goals {
		domainGoal, err := macroGoalFromRow(goals[i])
		if err != nil {
			return eaterdom.Eater{}, err
		}
		domainGoals = append(domainGoals, domainGoal)
	}
	profile.CurrentGoal = eaterdom.CurrentGoalAt(domainGoals, time.Now().UTC())
	return profile, nil
}

func (r *Eater) replaceAllergens(ctx context.Context, query *sqlc.Queries, eaterID eaterdom.ID, allergens []eaterdom.Allergen) error {
	if err := query.DeleteEaterAllergens(ctx, int64(eaterID)); err != nil {
		return mapCatalogError(err, "eater allergens")
	}
	for _, allergen := range allergens {
		if err := query.CreateEaterAllergen(ctx, sqlc.CreateEaterAllergenParams{EaterID: int64(eaterID), Allergen: string(allergen)}); err != nil {
			return mapCatalogError(err, "eater allergens")
		}
	}
	return nil
}

func (r *Eater) createGoal(ctx context.Context, query *sqlc.Queries, goal eaterdom.MacroGoal) (eaterdom.MacroGoal, error) {
	row, err := query.CreateMacroGoal(ctx, macroGoalParams(goal))
	if err != nil {
		return eaterdom.MacroGoal{}, mapCatalogError(err, "macro goal")
	}
	return macroGoalFromRow(row)
}

func (r *Eater) ensureEater(ctx context.Context, query *sqlc.Queries, id eaterdom.ID) error {
	_, err := query.GetEater(ctx, int64(id))
	return mapCatalogError(err, "eater")
}

func macroGoalParams(goal eaterdom.MacroGoal) sqlc.CreateMacroGoalParams {
	kcalMin := goal.Range.KcalMin
	kcalMax := goal.Range.KcalMax
	if kcalMin < 0 || int64(kcalMin) > math.MaxInt32 || kcalMax < 0 || int64(kcalMax) > math.MaxInt32 {
		panic("macro goal kcal must be a nonnegative int32")
	}
	return sqlc.CreateMacroGoalParams{
		EaterID: int64(goal.EaterID), EffectiveFrom: pgtype.Date{Time: goal.EffectiveFrom, Valid: true},
		ProteinMinG: numeric(goal.Range.ProteinMinG), ProteinMaxG: numeric(goal.Range.ProteinMaxG),
		FatMinG: numeric(goal.Range.FatMinG), FatMaxG: numeric(goal.Range.FatMaxG),
		CarbMinG: numeric(goal.Range.CarbMinG), CarbMaxG: numeric(goal.Range.CarbMaxG),
		KcalMin: int32(kcalMin), KcalMax: int32(kcalMax), Note: goal.Note,
	}
}

func macroGoalFromRow(row sqlc.EaterMacroGoal) (eaterdom.MacroGoal, error) {
	proteinMin, err := float64FromNumeric(row.ProteinMinG)
	if err != nil {
		return eaterdom.MacroGoal{}, err
	}
	proteinMax, err := float64FromNumeric(row.ProteinMaxG)
	if err != nil {
		return eaterdom.MacroGoal{}, err
	}
	fatMin, err := float64FromNumeric(row.FatMinG)
	if err != nil {
		return eaterdom.MacroGoal{}, err
	}
	fatMax, err := float64FromNumeric(row.FatMaxG)
	if err != nil {
		return eaterdom.MacroGoal{}, err
	}
	carbMin, err := float64FromNumeric(row.CarbMinG)
	if err != nil {
		return eaterdom.MacroGoal{}, err
	}
	carbMax, err := float64FromNumeric(row.CarbMaxG)
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
		ProteinG:           numeric(meal.Macros.ProteinG),
		FatG:               numeric(meal.Macros.FatG),
		CarbG:              numeric(meal.Macros.CarbG),
		Kcal:               int32(kcal),
		SaturatedFatG:      optionalNumeric(meal.Quality.SaturatedFatG),
		TransFatG:          optionalNumeric(meal.Quality.TransFatG),
		FiberG:             optionalNumeric(meal.Quality.FiberG),
		SodiumMg:           optionalNumeric(meal.Quality.SodiumMg),
		AddedSugarsG:       optionalNumeric(meal.Quality.AddedSugarsG),
		FreeSugarsG:        optionalNumeric(meal.Quality.FreeSugarsG),
		ProcessedMeatG:     optionalNumeric(meal.Quality.ProcessedMeatG),
		RedMeatG:           optionalNumeric(meal.Quality.RedMeatG),
		WholeGrainGEquiv:   optionalNumeric(meal.Quality.WholeGrainGEquiv),
		FruitVegG:          optionalNumeric(meal.Quality.FruitVegG),
		IncludeInAnalytics: meal.Quality.IncludeInAnalytics,
		IncludeInWarnings:  meal.Quality.IncludeInWarnings,
		Active:             meal.Active,
	}
}

func fixedMealFromRow(row sqlc.FixedMeal) (eaterdom.FixedMeal, error) {
	protein, err := float64FromNumeric(row.ProteinG)
	if err != nil {
		return eaterdom.FixedMeal{}, err
	}
	fat, err := float64FromNumeric(row.FatG)
	if err != nil {
		return eaterdom.FixedMeal{}, err
	}
	carb, err := float64FromNumeric(row.CarbG)
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
	saturated, err := optionalFloat64FromNumeric(row.SaturatedFatG)
	if err != nil {
		return eaterdom.FixedMealQuality{}, err
	}
	trans, err := optionalFloat64FromNumeric(row.TransFatG)
	if err != nil {
		return eaterdom.FixedMealQuality{}, err
	}
	fiber, err := optionalFloat64FromNumeric(row.FiberG)
	if err != nil {
		return eaterdom.FixedMealQuality{}, err
	}
	sodium, err := optionalFloat64FromNumeric(row.SodiumMg)
	if err != nil {
		return eaterdom.FixedMealQuality{}, err
	}
	added, err := optionalFloat64FromNumeric(row.AddedSugarsG)
	if err != nil {
		return eaterdom.FixedMealQuality{}, err
	}
	free, err := optionalFloat64FromNumeric(row.FreeSugarsG)
	if err != nil {
		return eaterdom.FixedMealQuality{}, err
	}
	processed, err := optionalFloat64FromNumeric(row.ProcessedMeatG)
	if err != nil {
		return eaterdom.FixedMealQuality{}, err
	}
	red, err := optionalFloat64FromNumeric(row.RedMeatG)
	if err != nil {
		return eaterdom.FixedMealQuality{}, err
	}
	wholeGrain, err := optionalFloat64FromNumeric(row.WholeGrainGEquiv)
	if err != nil {
		return eaterdom.FixedMealQuality{}, err
	}
	fruitVeg, err := optionalFloat64FromNumeric(row.FruitVegG)
	if err != nil {
		return eaterdom.FixedMealQuality{}, err
	}
	return eaterdom.FixedMealQuality{SaturatedFatG: saturated, TransFatG: trans, FiberG: fiber, SodiumMg: sodium, AddedSugarsG: added, FreeSugarsG: free, ProcessedMeatG: processed, RedMeatG: red, WholeGrainGEquiv: wholeGrain, FruitVegG: fruitVeg, IncludeInAnalytics: row.IncludeInAnalytics, IncludeInWarnings: row.IncludeInWarnings}, nil
}

func exclusionFromRow(row sqlc.EaterExclusion) eaterdom.Exclusion {
	return eaterdom.Exclusion{ID: eaterdom.ExclusionID(row.ID), EaterID: eaterdom.ID(row.EaterID), TargetKind: eaterdom.ExclusionTarget(row.TargetKind), TargetValue: row.TargetValue, Severity: eaterdom.ExclusionSeverity(row.Severity), Note: row.Note}
}
