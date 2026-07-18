package postgres

import (
	"context"

	"github.com/ZetoOfficial/meal-planner/internal/postgres/sqlc"
)

type executorContextKey struct{}

type Store struct {
	defaultQueries *sqlc.Queries
}

func NewStore(db sqlc.DBTX) *Store {
	return &Store{defaultQueries: sqlc.New(db)}
}

func (s *Store) Queries(ctx context.Context) *sqlc.Queries {
	if db, ok := ctx.Value(executorContextKey{}).(sqlc.DBTX); ok {
		return sqlc.New(db)
	}
	return s.defaultQueries
}

func withExecutor(ctx context.Context, db sqlc.DBTX) context.Context {
	return context.WithValue(ctx, executorContextKey{}, db)
}
