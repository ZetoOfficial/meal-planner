package planning

import (
	"testing"

	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

func storageOf(profiles map[recipedom.ID]recipedom.StorageProfile) func(recipedom.ID) (recipedom.StorageProfile, bool) {
	return func(id recipedom.ID) (recipedom.StorageProfile, bool) {
		profile, ok := profiles[id]
		return profile, ok
	}
}

func TestDerivePrepDependenciesLinksAnAssembledDishToItsCookedBatches(t *testing.T) {
	plan := Plan{
		Recipes: []PlanRecipe{{TemplateID: 1}, {TemplateID: 2}, {TemplateID: 3}},
		Meals: []Meal{
			{Components: []ComponentAllocation{{PlanRecipeIndex: 0}, {PlanRecipeIndex: 1}, {PlanRecipeIndex: 2}}},
			// The same pairing on another day must not duplicate the edge.
			{Components: []ComponentAllocation{{PlanRecipeIndex: 0}, {PlanRecipeIndex: 2}}},
		},
	}
	storage := storageOf(map[recipedom.ID]recipedom.StorageProfile{
		1: recipedom.StorageKeeps3d,
		2: recipedom.StorageKeeps3dReheat,
		3: recipedom.StorageAssembleFresh,
	})

	got := DerivePrepDependencies(plan, storage)

	if len(got) != 2 {
		t.Fatalf("dependencies = %#v, want the bowl to depend on both cooked batches once", got)
	}
	for _, dependency := range got {
		if dependency.DependentRecipeIndex != 2 || dependency.SourceRecipeIndex == 2 {
			t.Fatalf("dependency = %#v, want only edges from the assembled recipe 2", dependency)
		}
		if dependency.Note != nil {
			t.Fatalf("dependency = %#v, want no note: a derived edge cannot name a stored recipe id", dependency)
		}
	}
}

func TestDerivePrepDependenciesIgnoresBatchesEatenSideBySide(t *testing.T) {
	plan := Plan{
		Recipes: []PlanRecipe{{TemplateID: 1}, {TemplateID: 2}},
		Meals:   []Meal{{Components: []ComponentAllocation{{PlanRecipeIndex: 0}, {PlanRecipeIndex: 1}}}},
	}
	storage := storageOf(map[recipedom.ID]recipedom.StorageProfile{
		1: recipedom.StorageKeeps3d,
		2: recipedom.StorageKeeps3d,
	})

	if got := DerivePrepDependencies(plan, storage); len(got) != 0 {
		t.Fatalf("dependencies = %#v, want none between independently stored batches", got)
	}
}

func TestDerivePrepDependenciesSkipsUnknownTemplatesAndAssembledPeers(t *testing.T) {
	plan := Plan{
		Recipes: []PlanRecipe{{TemplateID: 1}, {TemplateID: 2}, {TemplateID: 9}},
		Meals: []Meal{{Components: []ComponentAllocation{
			{PlanRecipeIndex: 0}, {PlanRecipeIndex: 1}, {PlanRecipeIndex: 2}, {PlanRecipeIndex: 7},
		}}},
	}
	storage := storageOf(map[recipedom.ID]recipedom.StorageProfile{
		1: recipedom.StorageAssembleFresh,
		2: recipedom.StorageAssembleFresh,
	})

	if got := DerivePrepDependencies(plan, storage); len(got) != 0 {
		t.Fatalf("dependencies = %#v, want no edge to an unknown template or another assembled dish", got)
	}
}
