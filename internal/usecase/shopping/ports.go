package shopping

//go:generate go tool mockgen -source=$GOFILE -destination=mocks/ports.go -package=mocks

import (
	"context"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	planningdom "github.com/ZetoOfficial/meal-planner/internal/domain/planning"
	shoppingdom "github.com/ZetoOfficial/meal-planner/internal/domain/shopping"
)

type TxRunner interface {
	WithTx(ctx context.Context, fn func(context.Context) error) error
}

type ListRepository interface {
	Get(ctx context.Context, planID planningdom.PlanID) (shoppingdom.List, error)
	Save(ctx context.Context, list shoppingdom.List) (shoppingdom.List, error)
	SetChecked(ctx context.Context, itemID shoppingdom.ItemID, checked bool) error
	GetItem(ctx context.Context, itemID shoppingdom.ItemID) (shoppingdom.Item, error)
}

type PlanRepository interface {
	Get(ctx context.Context, id planningdom.PlanID) (planningdom.Plan, error)
	MarkShoppingListStale(ctx context.Context, planID planningdom.PlanID, stale bool) error
}

type ProductRepository interface {
	List(ctx context.Context, roleCode string) ([]catalogdom.Product, error)
}
