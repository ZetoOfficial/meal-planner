package catalog

//go:generate go tool mockgen -source=$GOFILE -destination=mocks/ports.go -package=mocks

import (
	"context"

	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
)

type TxRunner interface {
	WithTx(ctx context.Context, fn func(context.Context) error) error
}

type ProductRepository interface {
	Create(ctx context.Context, product catalogdom.Product) (catalogdom.Product, error)
	List(ctx context.Context, roleCode string) ([]catalogdom.Product, error)
	Get(ctx context.Context, id catalogdom.ProductID) (catalogdom.Product, error)
	Update(ctx context.Context, product catalogdom.Product) (catalogdom.Product, error)
	Delete(ctx context.Context, id catalogdom.ProductID) error
	GetRoleTag(ctx context.Context, code string) (catalogdom.RoleTag, error)
	ListRoleTags(ctx context.Context) ([]catalogdom.RoleTag, error)
}
