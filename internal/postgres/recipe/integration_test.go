//go:build integration

package recipe_test

import (
	"context"
	"testing"

	"github.com/ZetoOfficial/meal-planner/internal/clock"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
	postgresdb "github.com/ZetoOfficial/meal-planner/internal/postgres"
	reciperepo "github.com/ZetoOfficial/meal-planner/internal/postgres/recipe"
	testpostgres "github.com/ZetoOfficial/meal-planner/internal/testkit/postgres"
	recipeusecase "github.com/ZetoOfficial/meal-planner/internal/usecase/recipe"
)

func TestRecipeRoundTripPromotionAndPermanentCandidates(t *testing.T) {
	database := testpostgres.RequireDatabase(t)
	repository := reciperepo.New(postgresdb.NewStore(database.Pool))
	service := recipeusecase.NewService(repository, postgresdb.NewTxManager(database.Pool), clock.System{})
	ctx := context.Background()

	created, err := service.CreateRecipe(ctx, recipeusecase.CreateRecipeCommand{
		Name:          "Chicken bowl",
		Cuisine:       stringPointer("Mediterranean"),
		CookingMethod: stringPointer("roast"),
		Storage:       recipedom.StorageKeeps3dReheat,
		StorageNotes:  stringPointer("Keep sauce separate"),
		Equipment:     []string{"oven", "stove"},
		Instructions:  stringPointer("Roast, portion, and chill."),
		Slots: []recipeusecase.SlotCommand{
			{RoleTagCode: "protein:lean:breast", Label: "Protein", Qty: 180, Unit: "g", Position: 0},
			{RoleTagCode: "carb:grain", Label: "Grain", Qty: 200, Unit: "g", Position: 1},
		},
	})
	if err != nil {
		t.Fatalf("CreateRecipe() error = %v", err)
	}
	otherDraft, err := service.CreateRecipe(ctx, recipeusecase.CreateRecipeCommand{
		Name:    "Green vegetables",
		Storage: recipedom.StorageKeeps3d,
		Slots: []recipeusecase.SlotCommand{
			{RoleTagCode: "veg:green", Label: "Vegetables", Qty: 200, Unit: "g", Position: 0},
		},
	})
	if err != nil {
		t.Fatalf("CreateRecipe(other) error = %v", err)
	}

	loaded, err := service.GetRecipe(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetRecipe() error = %v", err)
	}
	if loaded.Source != recipedom.SourceManual || loaded.Status != recipedom.StatusDraft || len(loaded.Slots) != 2 {
		t.Fatalf("GetRecipe() = %#v, want manual draft with two slots", loaded)
	}
	if loaded.Cuisine == nil || *loaded.Cuisine != "Mediterranean" || len(loaded.Equipment) != 2 {
		t.Fatalf("GetRecipe() metadata = %#v/%#v", loaded.Cuisine, loaded.Equipment)
	}

	draftStatus := recipedom.StatusDraft
	drafts, err := service.ListRecipes(ctx, &draftStatus)
	if err != nil || len(drafts) != 2 {
		t.Fatalf("ListRecipes(draft) = %#v, %v", drafts, err)
	}

	promoted, err := service.PromoteRecipe(ctx, created.ID)
	if err != nil {
		t.Fatalf("PromoteRecipe() error = %v", err)
	}
	if promoted.Status != recipedom.StatusPermanent || promoted.CookedCount != 1 || promoted.PromotedAt == nil {
		t.Fatalf("PromoteRecipe() = %#v", promoted)
	}

	candidates, err := service.ListPermanentCandidates(ctx)
	if err != nil {
		t.Fatalf("ListPermanentCandidates() error = %v", err)
	}
	if len(candidates) != 1 || candidates[0].ID != created.ID || len(candidates[0].Slots) != 2 {
		t.Fatalf("ListPermanentCandidates() = %#v, want only promoted recipe", candidates)
	}
	if candidates[0].ID == otherDraft.ID {
		t.Fatal("draft recipe returned as permanent candidate")
	}
}

func stringPointer(value string) *string { return &value }
