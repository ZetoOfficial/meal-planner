package repositories

import (
	"context"

	"github.com/ZetoOfficial/meal-planner/internal/postgres/sqlc"
)

type executorContextKey struct{}

type Repositories struct {
	defaultQueries *sqlc.Queries
}

func New(db sqlc.DBTX) *Repositories {
	return &Repositories{
		defaultQueries: sqlc.New(db),
	}
}

func WithExecutor(ctx context.Context, db sqlc.DBTX) context.Context {
	return context.WithValue(ctx, executorContextKey{}, db)
}

func (r *Repositories) Queries(ctx context.Context) *sqlc.Queries {
	if db, ok := ctx.Value(executorContextKey{}).(sqlc.DBTX); ok {
		return sqlc.New(db)
	}
	return r.defaultQueries
}
