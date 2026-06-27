// Package catalog (usecase) — сценарии работы с каталогом продуктов.
// Интерфейсы (ports) живут на стороне потребителя (ARCHITECTURE §7).
package catalog

import (
	"context"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
)

// ProductRepository — порт доступа к продуктам (реализуется postgres-адаптером).
type ProductRepository interface {
	ListUsable(ctx context.Context) ([]catalogdom.Product, error)
	Get(ctx context.Context, id catalogdom.ProductID) (catalogdom.Product, error)
	Save(ctx context.Context, p catalogdom.Product) error
}
