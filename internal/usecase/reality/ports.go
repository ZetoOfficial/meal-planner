package reality

//go:generate go tool mockgen -source=$GOFILE -destination=mocks/ports.go -package=mocks

import (
	"context"
	"time"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	feedbackdom "github.com/ZetoOfficial/meal-planner/internal/domain/feedback"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

type Clock interface {
	Now() time.Time
}

type TxRunner interface {
	WithTx(ctx context.Context, fn func(context.Context) error) error
}

type MealRepository interface {
	ListPlanMeals(ctx context.Context, planID planningdom.PlanID, eaterID *eaterdom.ID) ([]planningdom.Meal, error)
	ListEaterMeals(ctx context.Context, eaterID eaterdom.ID, from, to time.Time) ([]planningdom.Meal, error)
	GetMeal(ctx context.Context, mealID planningdom.MealID) (planningdom.Meal, error)
	LogMeal(ctx context.Context, mealID planningdom.MealID, log planningdom.MealLog) (planningdom.Meal, error)
	ListPlansForEaterPeriod(ctx context.Context, eaterID eaterdom.ID, from, to time.Time) ([]planningdom.Plan, error)
	SaveQualitySnapshots(ctx context.Context, planID planningdom.PlanID, snapshots []planningdom.QualitySnapshot) error
}

// CatalogueRepository serves the reference data an actual-basis recomputation
// needs to read a stored plan back as quality input.
type CatalogueRepository interface {
	ListProducts(ctx context.Context) ([]catalogdom.Product, error)
	ListRecipeTemplates(ctx context.Context) ([]recipedom.Template, error)
	GetEater(ctx context.Context, id eaterdom.ID) (eaterdom.Eater, error)
	ListFixedMeals(ctx context.Context, eaterID eaterdom.ID) ([]eaterdom.FixedMeal, error)
}

type FeedbackRepository interface {
	Create(ctx context.Context, event feedbackdom.Event) (feedbackdom.Event, error)
	AdjustRecipeWeight(ctx context.Context, recipeID recipedom.ID, delta float64) error
}
