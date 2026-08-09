//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

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

func TestMigrationsInstallPlanningQualityContracts(t *testing.T) {
	db := testpostgres.RequireDatabase(t)

	columns := []struct {
		table    string
		column   string
		nullable bool
	}{
		{table: "plan_eater_day", column: "generated_meal_types"},
		{table: "meal_component_allocation", column: "portion_factor"},
		{table: "plan_recipe", column: "scale_factor"},
		{table: "slot_fill", column: "planned_input_qty_g"},
		{table: "fixed_meal", column: "grain_g_equiv", nullable: true},
		{table: "fixed_meal", column: "legume_g", nullable: true},
		{table: "fixed_meal", column: "upf_kcal", nullable: true},
		{table: "day_nutrition_summary", column: "data_coverage_score"},
		{table: "rolling_nutrition_summary", column: "data_coverage_score"},
		{table: "product_food_classification", column: "food_group", nullable: true},
		{table: "product_food_classification", column: "is_legume", nullable: true},
		{table: "rule_evaluation_result", column: "coverage_score"},
	}

	for _, column := range columns {
		t.Run(column.table+"."+column.column, func(t *testing.T) {
			var nullable string
			err := db.Pool.QueryRow(t.Context(), `
				SELECT is_nullable
				FROM information_schema.columns
				WHERE table_schema = current_schema()
				  AND table_name = $1
				  AND column_name = $2
			`, column.table, column.column).Scan(&nullable)
			if err != nil {
				t.Fatalf("read migrated column: %v", err)
			}
			if got := nullable == "YES"; got != column.nullable {
				t.Fatalf("nullable = %t, want %t", got, column.nullable)
			}
		})
	}
}

func TestMigrationsMatchCanonicalSchemaContract(t *testing.T) {
	db := testpostgres.RequireDatabase(t)
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve integration test path")
	}
	schemaSQL, err := os.ReadFile(filepath.Join(filepath.Dir(testFile), "..", "..", "docs", "schema.sql"))
	if err != nil {
		t.Fatalf("read canonical schema: %v", err)
	}

	tx, err := db.Pool.Begin(t.Context())
	if err != nil {
		t.Fatalf("begin canonical schema transaction: %v", err)
	}
	defer func() {
		if err := tx.Rollback(t.Context()); err != nil && err != pgx.ErrTxClosed {
			t.Errorf("rollback canonical schema transaction: %v", err)
		}
	}()

	suffix := db.Schema
	if len(suffix) > 24 {
		suffix = suffix[len(suffix)-24:]
	}
	canonicalSchema := "canonical_" + suffix
	quotedCanonical := pgx.Identifier{canonicalSchema}.Sanitize()
	if _, err := tx.Exec(t.Context(), "CREATE SCHEMA "+quotedCanonical); err != nil {
		t.Fatalf("create canonical schema: %v", err)
	}
	if _, err := tx.Exec(t.Context(), "SET LOCAL search_path = "+quotedCanonical); err != nil {
		t.Fatalf("select canonical schema: %v", err)
	}
	if _, err := tx.Exec(t.Context(), string(schemaSQL)); err != nil {
		t.Fatalf("apply canonical schema: %v", err)
	}

	for _, contract := range []struct {
		name  string
		query string
	}{
		{
			name: "columns",
			query: `
				SELECT table_name, column_name, udt_name, is_nullable
				FROM information_schema.columns
				WHERE table_schema = $1
				ORDER BY table_name, column_name
			`,
		},
		{
			name: "enums",
			query: `
				SELECT t.typname, e.enumlabel, e.enumsortorder::text
				FROM pg_type t
				JOIN pg_enum e ON e.enumtypid = t.oid
				JOIN pg_namespace n ON n.oid = t.typnamespace
				WHERE n.nspname = $1
				ORDER BY t.typname, e.enumsortorder
			`,
		},
		{
			name: "constraints",
			query: `
				SELECT c.relname, con.contype::text, pg_get_constraintdef(con.oid)
				FROM pg_constraint con
				JOIN pg_class c ON c.oid = con.conrelid
				JOIN pg_namespace n ON n.oid = c.relnamespace
				WHERE n.nspname = $1
				ORDER BY c.relname, con.contype, pg_get_constraintdef(con.oid)
			`,
		},
	} {
		t.Run(contract.name, func(t *testing.T) {
			migrated := contractRows(t, tx, contract.query, db.Schema)
			canonical := contractRows(t, tx, contract.query, canonicalSchema)
			if !reflect.DeepEqual(migrated, canonical) {
				t.Fatalf("migration contract differs from docs/schema.sql\nmigrated:\n%s\ncanonical:\n%s", strings.Join(migrated, "\n"), strings.Join(canonical, "\n"))
			}
		})
	}
}

func TestPlanningQualityContractMigrationDown(t *testing.T) {
	db := testpostgres.RequireDatabase(t)
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve integration test path")
	}
	migration, err := os.ReadFile(filepath.Join(filepath.Dir(testFile), "..", "..", "migrations", "004_planning_quality_contract_fixes.sql"))
	if err != nil {
		t.Fatalf("read planning contract migration: %v", err)
	}
	_, down, found := strings.Cut(string(migration), "---- create above / drop below ----")
	if !found {
		t.Fatal("planning contract migration has no down separator")
	}

	tx, err := db.Pool.Begin(t.Context())
	if err != nil {
		t.Fatalf("begin migration down transaction: %v", err)
	}
	defer func() {
		if err := tx.Rollback(t.Context()); err != nil && err != pgx.ErrTxClosed {
			t.Errorf("rollback migration down transaction: %v", err)
		}
	}()
	if _, err := tx.Exec(t.Context(), down); err != nil {
		t.Fatalf("apply planning contract down migration: %v", err)
	}
}

func contractRows(t *testing.T, tx pgx.Tx, query, schema string) []string {
	t.Helper()
	rows, err := tx.Query(t.Context(), query, schema)
	if err != nil {
		t.Fatalf("query %s contract: %v", schema, err)
	}
	defer rows.Close()

	result := make([]string, 0)
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			t.Fatalf("read %s contract row: %v", schema, err)
		}
		parts := make([]string, len(values))
		for i := range values {
			parts[i] = fmt.Sprint(values[i])
		}
		row := strings.Join(parts, "|")
		result = append(result, strings.ReplaceAll(row, schema+".", ""))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate %s contract: %v", schema, err)
	}
	sort.Strings(result)
	return result
}
