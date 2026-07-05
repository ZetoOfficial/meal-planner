// Package repositories — адаптеры Postgres: маппинг sqlc rows ↔ domain.
// sqlc-типы НЕ должны протекать в domain/usecase/solver/quality (ARCHITECTURE §10).
package repositories

import (
	"context"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/sqlc"
)

// ProductRepository реализует соответствующий usecase-порт поверх sqlc.
type ProductRepository struct {
	q sqlc.Querier
}

func NewProductRepository(q sqlc.Querier) *ProductRepository {
	return &ProductRepository{q: q}
}

func (r *ProductRepository) ListUsable(ctx context.Context) ([]catalogdom.Product, error) {
	rows, err := r.q.ListProducts(ctx)
	if err != nil {
		return nil, err
	}
	products := make([]catalogdom.Product, 0, len(rows))
	for i := range rows {
		products = append(products, productFromRow(rows[i]))
	}
	return products, nil
}

func (r *ProductRepository) Get(ctx context.Context, id catalogdom.ProductID) (catalogdom.Product, error) {
	row, err := r.q.GetProduct(ctx, int64(id))
	if err != nil {
		return catalogdom.Product{}, err
	}
	return productFromRow(row), nil
}

func (r *ProductRepository) Save(context.Context, catalogdom.Product) error {
	return ErrNotImplemented
}

func productFromRow(row sqlc.Product) catalogdom.Product {
	return catalogdom.Product{
		ID:    catalogdom.ProductID(row.ID),
		Brand: row.Brand,
		Name:  row.Name,
	}
}
