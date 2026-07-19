package recipe

//go:generate go tool mockgen -source=$GOFILE -destination=mocks/ports.go -package=mocks

import (
	"context"
	"time"

	"github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

type Clock interface {
	Now() time.Time
}

type TxRunner interface {
	WithTx(ctx context.Context, fn func(context.Context) error) error
}

type RecipeRepository interface {
	ResolveRoleTags(ctx context.Context, codes []string) (map[string]catalog.RoleTag, error)
	Create(ctx context.Context, template recipedom.Template, roles map[string]catalog.RoleTag) (recipedom.Template, error)
	List(ctx context.Context, status *recipedom.Status) ([]recipedom.Template, error)
	Get(ctx context.Context, id recipedom.ID) (recipedom.Template, error)
	Promote(ctx context.Context, id recipedom.ID, promotedAt time.Time) (recipedom.Template, error)
	ListPermanentCandidates(ctx context.Context) ([]recipedom.Template, error)
}
