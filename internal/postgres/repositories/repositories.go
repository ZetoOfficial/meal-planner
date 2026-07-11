// Package repositories contains Postgres/sqlc infrastructure wiring.
package repositories

import (
	"context"

	"github.com/ZetoOfficial/meal-planner/internal/postgres/sqlc"
)

type executorContextKey struct{}

// Repositories owns the sqlc query set used by future concrete adapters.
// It deliberately does not implement any use-case repository port.
type Repositories struct {
	defaultQueries *sqlc.Queries
}

// New creates a query set backed by db, which may be a pool or a transaction.
func New(db sqlc.DBTX) *Repositories {
	return &Repositories{
		defaultQueries: sqlc.New(db),
	}
}

// WithExecutor binds db to ctx for adapter code that runs inside a transaction.
func WithExecutor(ctx context.Context, db sqlc.DBTX) context.Context {
	return context.WithValue(ctx, executorContextKey{}, db)
}

// Queries returns generated queries backed by the context executor or default db.
func (r *Repositories) Queries(ctx context.Context) *sqlc.Queries {
	if db, ok := ctx.Value(executorContextKey{}).(sqlc.DBTX); ok {
		return sqlc.New(db)
	}
	return r.defaultQueries
}
