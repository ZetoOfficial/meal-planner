package catalog

import (
	"context"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
)

type ProductRepository interface {
	ListUsable(ctx context.Context) ([]catalogdom.Product, error)
	Get(ctx context.Context, id catalogdom.ProductID) (catalogdom.Product, error)
	Save(ctx context.Context, p catalogdom.Product) error
}
