package eater

import (
	"context"
	"time"

	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	"github.com/ZetoOfficial/meal-planner/internal/postgres"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/dberror"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/sqlc"
)

type Repository struct {
	store *postgres.Store
}

func New(store *postgres.Store) *Repository {
	return &Repository{store: store}
}

func (r *Repository) Create(ctx context.Context, profile eaterdom.Eater, goal eaterdom.MacroGoal) (eaterdom.Eater, error) {
	query := r.store.Queries(ctx)
	row, err := query.CreateEater(ctx, sqlc.CreateEaterParams{Name: profile.Name, Sex: sqlc.Sex(profile.Sex)})
	if err != nil {
		return eaterdom.Eater{}, dberror.Map(err, "eater")
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

func (r *Repository) List(ctx context.Context) ([]eaterdom.Eater, error) {
	query := r.store.Queries(ctx)
	rows, err := query.ListEaters(ctx)
	if err != nil {
		return nil, dberror.Map(err, "eater")
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

func (r *Repository) Get(ctx context.Context, id eaterdom.ID) (eaterdom.Eater, error) {
	return r.load(ctx, r.store.Queries(ctx), id)
}

func (r *Repository) Update(ctx context.Context, profile eaterdom.Eater, goal eaterdom.MacroGoal) (eaterdom.Eater, error) {
	query := r.store.Queries(ctx)
	if _, err := query.UpdateEater(ctx, sqlc.UpdateEaterParams{ID: int64(profile.ID), Name: profile.Name, Sex: sqlc.Sex(profile.Sex)}); err != nil {
		return eaterdom.Eater{}, dberror.Map(err, "eater")
	}
	if err := r.replaceAllergens(ctx, query, profile.ID, profile.Allergens); err != nil {
		return eaterdom.Eater{}, err
	}
	if _, err := r.createGoal(ctx, query, goal); err != nil {
		return eaterdom.Eater{}, err
	}
	return r.load(ctx, query, profile.ID)
}

func (r *Repository) ListGoals(ctx context.Context, eaterID eaterdom.ID) ([]eaterdom.MacroGoal, error) {
	query := r.store.Queries(ctx)
	if err := r.ensureEater(ctx, query, eaterID); err != nil {
		return nil, err
	}
	rows, err := query.ListMacroGoals(ctx, int64(eaterID))
	if err != nil {
		return nil, dberror.Map(err, "macro goal")
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

func (r *Repository) CreateGoal(ctx context.Context, goal eaterdom.MacroGoal) (eaterdom.MacroGoal, error) {
	query := r.store.Queries(ctx)
	if err := r.ensureEater(ctx, query, goal.EaterID); err != nil {
		return eaterdom.MacroGoal{}, err
	}
	return r.createGoal(ctx, query, goal)
}

func (r *Repository) ListFixedMeals(ctx context.Context, eaterID eaterdom.ID) ([]eaterdom.FixedMeal, error) {
	query := r.store.Queries(ctx)
	if err := r.ensureEater(ctx, query, eaterID); err != nil {
		return nil, err
	}
	rows, err := query.ListActiveFixedMeals(ctx, int64(eaterID))
	if err != nil {
		return nil, dberror.Map(err, "fixed meal")
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

func (r *Repository) CreateFixedMeal(ctx context.Context, meal eaterdom.FixedMeal) (eaterdom.FixedMeal, error) {
	query := r.store.Queries(ctx)
	if err := r.ensureEater(ctx, query, meal.EaterID); err != nil {
		return eaterdom.FixedMeal{}, err
	}
	row, err := query.CreateFixedMeal(ctx, fixedMealParams(meal))
	if err != nil {
		return eaterdom.FixedMeal{}, dberror.Map(err, "fixed meal")
	}
	return fixedMealFromRow(row)
}

func (r *Repository) ListExclusions(ctx context.Context, eaterID eaterdom.ID) ([]eaterdom.Exclusion, error) {
	query := r.store.Queries(ctx)
	if err := r.ensureEater(ctx, query, eaterID); err != nil {
		return nil, err
	}
	rows, err := query.ListEaterExclusions(ctx, int64(eaterID))
	if err != nil {
		return nil, dberror.Map(err, "exclusion")
	}
	exclusions := make([]eaterdom.Exclusion, 0, len(rows))
	for i := range rows {
		exclusions = append(exclusions, exclusionFromRow(rows[i]))
	}
	return exclusions, nil
}

func (r *Repository) CreateExclusion(ctx context.Context, exclusion eaterdom.Exclusion) (eaterdom.Exclusion, error) {
	query := r.store.Queries(ctx)
	if err := r.ensureEater(ctx, query, exclusion.EaterID); err != nil {
		return eaterdom.Exclusion{}, err
	}
	row, err := query.CreateEaterExclusion(ctx, sqlc.CreateEaterExclusionParams{
		EaterID: int64(exclusion.EaterID), TargetKind: sqlc.ExclusionTarget(exclusion.TargetKind), TargetValue: exclusion.TargetValue,
		Severity: sqlc.ExclusionSeverity(exclusion.Severity), Note: exclusion.Note,
	})
	if err != nil {
		return eaterdom.Exclusion{}, dberror.Map(err, "exclusion")
	}
	return exclusionFromRow(row), nil
}

func (r *Repository) DeleteExclusion(ctx context.Context, id eaterdom.ExclusionID) error {
	if _, err := r.store.Queries(ctx).DeleteEaterExclusion(ctx, int64(id)); err != nil {
		return dberror.Map(err, "exclusion")
	}
	return nil
}

func (r *Repository) RoleTagExists(ctx context.Context, code string) (bool, error) {
	exists, err := r.store.Queries(ctx).RoleTagExists(ctx, code)
	return exists, dberror.Map(err, "role tag")
}

func (r *Repository) ProductExists(ctx context.Context, id int64) (bool, error) {
	exists, err := r.store.Queries(ctx).ProductExists(ctx, id)
	return exists, dberror.Map(err, "product")
}

func (r *Repository) load(ctx context.Context, query *sqlc.Queries, id eaterdom.ID) (eaterdom.Eater, error) {
	row, err := query.GetEater(ctx, int64(id))
	if err != nil {
		return eaterdom.Eater{}, dberror.Map(err, "eater")
	}
	profile := eaterdom.Eater{ID: eaterdom.ID(row.ID), Name: row.Name, Sex: eaterdom.Sex(row.Sex)}
	allergens, err := query.ListEaterAllergens(ctx, int64(id))
	if err != nil {
		return eaterdom.Eater{}, dberror.Map(err, "eater allergens")
	}
	profile.Allergens = make([]eaterdom.Allergen, len(allergens))
	for i, allergen := range allergens {
		profile.Allergens[i] = eaterdom.Allergen(allergen)
	}
	exclusions, err := query.ListEaterExclusions(ctx, int64(id))
	if err != nil {
		return eaterdom.Eater{}, dberror.Map(err, "exclusion")
	}
	profile.Exclusions = make([]eaterdom.Exclusion, 0, len(exclusions))
	for _, exclusion := range exclusions {
		profile.Exclusions = append(profile.Exclusions, exclusionFromRow(exclusion))
	}
	goals, err := query.ListMacroGoals(ctx, int64(id))
	if err != nil {
		return eaterdom.Eater{}, dberror.Map(err, "macro goal")
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

func (r *Repository) replaceAllergens(ctx context.Context, query *sqlc.Queries, eaterID eaterdom.ID, allergens []eaterdom.Allergen) error {
	if err := query.DeleteEaterAllergens(ctx, int64(eaterID)); err != nil {
		return dberror.Map(err, "eater allergens")
	}
	for _, allergen := range allergens {
		if err := query.CreateEaterAllergen(ctx, sqlc.CreateEaterAllergenParams{EaterID: int64(eaterID), Allergen: string(allergen)}); err != nil {
			return dberror.Map(err, "eater allergens")
		}
	}
	return nil
}

func (r *Repository) createGoal(ctx context.Context, query *sqlc.Queries, goal eaterdom.MacroGoal) (eaterdom.MacroGoal, error) {
	row, err := query.CreateMacroGoal(ctx, macroGoalParams(goal))
	if err != nil {
		return eaterdom.MacroGoal{}, dberror.Map(err, "macro goal")
	}
	return macroGoalFromRow(row)
}

func (r *Repository) ensureEater(ctx context.Context, query *sqlc.Queries, id eaterdom.ID) error {
	_, err := query.GetEater(ctx, int64(id))
	return dberror.Map(err, "eater")
}
