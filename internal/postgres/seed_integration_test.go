//go:build integration

package postgres_test

import (
	"context"
	"testing"

	testpostgres "github.com/ZetoOfficial/meal-planner/internal/testkit/postgres"
)

func TestDevSeedProducesAUsefulDemoState(t *testing.T) {
	database := testpostgres.RequireDatabase(t)
	testpostgres.ApplySeed(t, database)
	ctx := context.Background()

	counts := map[string]int{
		"eater":                       2,
		"eater_macro_goal":            2,
		"fixed_meal":                  1,
		"eater_allergen":              1,
		"eater_exclusion":             1,
		"recipe_template":             5,
		"product_food_classification": 9,
	}
	for table, want := range counts {
		var got int
		if err := database.Pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&got); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if got != want {
			t.Fatalf("%s rows = %d, want %d", table, got, want)
		}
	}

	var products int
	if err := database.Pool.QueryRow(ctx, "SELECT count(*) FROM product").Scan(&products); err != nil {
		t.Fatalf("count products: %v", err)
	}
	if products < 9 {
		t.Fatalf("products = %d, want a realistic fixture set", products)
	}

	// Every seeded recipe slot must be fillable, otherwise a demo plan cannot
	// be generated at all.
	var unfillable int
	err := database.Pool.QueryRow(ctx, `
		SELECT count(*)
		FROM recipe_slot rs
		WHERE NOT EXISTS (
			SELECT 1 FROM product_role pr WHERE pr.role_tag_id = rs.role_tag_id
		)
	`).Scan(&unfillable)
	if err != nil {
		t.Fatalf("check fillable slots: %v", err)
	}
	if unfillable != 0 {
		t.Fatalf("unfillable recipe slots = %d, want every seeded slot to have a product", unfillable)
	}

	// Sasha is allergic to milk, so no seeded product she could eat may carry it.
	var conflicts int
	err = database.Pool.QueryRow(ctx, `
		SELECT count(*)
		FROM product_allergen pa
		JOIN eater_allergen ea ON ea.allergen = pa.allergen
	`).Scan(&conflicts)
	if err != nil {
		t.Fatalf("check allergen conflicts: %v", err)
	}
	if conflicts != 0 {
		t.Fatalf("seeded allergen conflicts = %d, want the fixture to respect known allergens", conflicts)
	}

	// One product is deliberately sparse so quality warnings and unknown
	// verdicts are both reachable from the demo state.
	var sparse int
	err = database.Pool.QueryRow(ctx, `
		SELECT count(*) FROM product WHERE fiber_per_100g IS NULL
	`).Scan(&sparse)
	if err != nil {
		t.Fatalf("check sparse products: %v", err)
	}
	if sparse == 0 {
		t.Fatal("every seeded product has full nutrition data, want an unknown-coverage case too")
	}
}
