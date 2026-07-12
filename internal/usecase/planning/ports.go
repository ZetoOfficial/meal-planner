package planning

import (
	"context"

	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

type PlanRepository interface {
	Save(ctx context.Context, p any) error
	Get(ctx context.Context, id int64) (any, error)
}

type RecipeGenerator interface {
	Generate(ctx context.Context, input any) ([]recipedom.TemplateDraft, error)
}

type TxManager interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}
