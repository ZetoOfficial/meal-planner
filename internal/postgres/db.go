// Package postgres — инфраструктурный адаптер: подключение, транзакции, sqlc.
package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool открывает пул соединений к Postgres по DSN.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, dsn)
}
