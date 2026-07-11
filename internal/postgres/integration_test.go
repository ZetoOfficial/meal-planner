//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/ZetoOfficial/meal-planner/internal/postgres/postgrestest"
)

func TestMigrationsCreateProductTableInIsolatedSchema(t *testing.T) {
	db := postgrestest.RequireDatabase(t)
	assertOnlyIsolatedSchema(t, db)

	var exists bool
	err := db.Pool.QueryRow(
		context.Background(),
		`SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = $1 AND table_name = 'product'
		)`,
		db.Schema,
	).Scan(&exists)
	if err != nil {
		t.Fatalf("query product table in schema %q: %v", db.Schema, err)
	}
	if !exists {
		t.Fatalf("product table does not exist in isolated schema %q", db.Schema)
	}
}

func assertOnlyIsolatedSchema(t *testing.T, db postgrestest.Database) {
	t.Helper()

	var schemas []string
	if err := db.Pool.QueryRow(context.Background(), `SELECT current_schemas(false)`).Scan(&schemas); err != nil {
		t.Fatalf("query current schemas: %v", err)
	}
	if len(schemas) != 1 || schemas[0] != db.Schema {
		t.Fatalf("current schemas = %q, want only isolated schema %q", schemas, db.Schema)
	}
}
