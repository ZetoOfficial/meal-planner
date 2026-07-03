//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/ZetoOfficial/meal-planner/internal/postgres/postgrestest"
)

func TestIntegrationMigrationsCreateIsolatedSchema(t *testing.T) {
	db := postgrestest.RequireDatabase(t)

	var exists bool
	err := db.Pool.QueryRow(context.Background(), `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = $1
			  AND table_name = 'product'
		 )
	`, db.Schema).Scan(&exists)
	if err != nil {
		t.Fatalf("check migrated product table: %v", err)
	}
	if !exists {
		t.Fatalf("product table was not created in isolated schema %q", db.Schema)
	}
}
