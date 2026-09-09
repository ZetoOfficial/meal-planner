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

	SwapSlotFill(ctx context.Context, planID planningdom.PlanID, fillID planningdom.SlotFillID, productID catalogdom.ProductID) error
	MarkShoppingListStale(ctx context.Context, planID planningdom.PlanID, stale bool) error

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
	ListRoleTags(ctx context.Context) ([]catalogdom.RoleTag, error)
}

// RecipeRepository serves the reuse-first candidate set (permanent recipes) and
// stores accepted LLM candidates as drafts.
type RecipeRepository interface {
	ListPermanentCandidates(ctx context.Context) ([]recipedom.Template, error)
	List(ctx context.Context, status *recipedom.Status) ([]recipedom.Template, error)
	ResolveRoleTags(ctx context.Context, codes []string) (map[string]catalogdom.RoleTag, error)
	Create(ctx context.Context, template recipedom.Template, roles map[string]catalogdom.RoleTag) (recipedom.Template, error)
}

type Solver interface {
	BuildPlan(ctx context.Context, input solver.Input) (solver.Result, error)
}
