package eater

import (
	"context"

	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
)

type TxRunner interface {
	WithTx(ctx context.Context, fn func(context.Context) error) error
}

type EaterRepository interface {
	Create(ctx context.Context, profile eaterdom.Eater, goal eaterdom.MacroGoal) (eaterdom.Eater, error)
	List(ctx context.Context) ([]eaterdom.Eater, error)
	Get(ctx context.Context, id eaterdom.ID) (eaterdom.Eater, error)
	Update(ctx context.Context, profile eaterdom.Eater, goal eaterdom.MacroGoal) (eaterdom.Eater, error)

	ListGoals(ctx context.Context, eaterID eaterdom.ID) ([]eaterdom.MacroGoal, error)
	CreateGoal(ctx context.Context, goal eaterdom.MacroGoal) (eaterdom.MacroGoal, error)

	ListFixedMeals(ctx context.Context, eaterID eaterdom.ID) ([]eaterdom.FixedMeal, error)
	CreateFixedMeal(ctx context.Context, meal eaterdom.FixedMeal) (eaterdom.FixedMeal, error)

	ListExclusions(ctx context.Context, eaterID eaterdom.ID) ([]eaterdom.Exclusion, error)
	CreateExclusion(ctx context.Context, exclusion eaterdom.Exclusion) (eaterdom.Exclusion, error)
	DeleteExclusion(ctx context.Context, id eaterdom.ExclusionID) error

	RoleTagExists(ctx context.Context, code string) (bool, error)
	ProductExists(ctx context.Context, id int64) (bool, error)
}
