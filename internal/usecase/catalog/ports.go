package catalog

import (
	"context"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
)

type ProductFilter struct {
	Role *string
}

type ProductRepository interface {
	List(ctx context.Context, filter ProductFilter) ([]catalogdom.Product, error)
	ListUsable(ctx context.Context) ([]catalogdom.Product, error)
	Get(ctx context.Context, id catalogdom.ProductID) (catalogdom.Product, error)
	Create(ctx context.Context, p catalogdom.Product) (catalogdom.Product, error)
	Update(ctx context.Context, p catalogdom.Product) (catalogdom.Product, error)
	Delete(ctx context.Context, id catalogdom.ProductID) error
	ListRoleTags(ctx context.Context) ([]catalogdom.RoleTag, error)
}
