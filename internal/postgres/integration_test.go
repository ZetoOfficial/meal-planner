//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/ZetoOfficial/meal-planner/internal/postgres/sqlc"
	testpostgres "github.com/ZetoOfficial/meal-planner/internal/testkit/postgres"
)

func TestMigrationsCreateProductTableInIsolatedSchema(t *testing.T) {
	db := testpostgres.RequireDatabase(t)

	_, err := sqlc.New(db.Pool).ListProducts(context.Background())
	if err != nil {
		t.Fatalf("list products after migrations: %v", err)
	}
}
