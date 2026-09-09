package planning

//go:generate go tool mockgen -source=$GOFILE -destination=mocks/ports.go -package=mocks

import (
	"context"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
	"github.com/ZetoOfficial/meal-planner/internal/solver"
)

type TxRunner interface {
	WithTx(ctx context.Context, fn func(context.Context) error) error
}

// PlanRepository persists the plan aggregate. Save expects to run inside the
// caller's transaction so a failed step leaves no partial plan behind.
type PlanRepository interface {
	Save(ctx context.Context, plan planningdom.Plan) (planningdom.Plan, error)
	Get(ctx context.Context, id planningdom.PlanID) (planningdom.Plan, error)

	List(ctx context.Context) ([]planningdom.Plan, error)
	Archive(ctx context.Context, id planningdom.PlanID) error

	SaveQualitySnapshots(ctx context.Context, planID planningdom.PlanID, snapshots []planningdom.QualitySnapshot) error
	ListQualitySnapshots(ctx context.Context, planID planningdom.PlanID, basis nutrition.SummaryBasis) ([]planningdom.QualitySnapshot, error)
}

// EaterRepository supplies the planning inputs of one eater: profile with
// allergens and exclusions, the goal history, and the fixed meals.
type EaterRepository interface {
	Get(ctx context.Context, id eaterdom.ID) (eaterdom.Eater, error)
	ListGoals(ctx context.Context, eaterID eaterdom.ID) ([]eaterdom.MacroGoal, error)
	ListFixedMeals(ctx context.Context, eaterID eaterdom.ID) ([]eaterdom.FixedMeal, error)
}

type ProductRepository interface {
	List(ctx context.Context, roleCode string) ([]catalogdom.Product, error)
}

// RecipeRepository serves the reuse-first candidate set: permanent recipes only.
type RecipeRepository interface {
	ListPermanentCandidates(ctx context.Context) ([]recipedom.Template, error)
}

type Solver interface {
	BuildPlan(ctx context.Context, input solver.Input) (solver.Result, error)
}
