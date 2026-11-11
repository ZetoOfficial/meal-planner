package planning

import (
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

// PrepDependency records that one batch is put together from another: the
// dependent recipe uses the source recipe's output. Swapping the source breaks
// the dependent dish, so the graph must be visible before a swap is applied.
type PrepDependency struct {
	ID int64
	// The indexes address Plan.Recipes, like component allocations do, so the
	// graph can be built before the plan has database ids.
	DependentRecipeIndex int
	SourceRecipeIndex    int
	Note                 *string
}

// DerivePrepDependencies builds the prep graph from what the plan already
// knows. An assemble_fresh batch is not cooked ahead: it is put together at
// mealtime from the cooked batches it is served with, so it depends on every
// stored batch that shares one of its meals. Batches that are simply eaten side
// by side do not depend on each other.
//
// storage reports the storage profile of a template; a template it does not
// know contributes no edge.
func DerivePrepDependencies(
	plan Plan,
	storage func(recipedom.ID) (recipedom.StorageProfile, bool),
) []PrepDependency {
	assembled := make([]bool, len(plan.Recipes))
	known := make([]bool, len(plan.Recipes))
	for i := range plan.Recipes {
		profile, ok := storage(plan.Recipes[i].TemplateID)
		known[i] = ok
		assembled[i] = ok && profile == recipedom.StorageAssembleFresh
	}

	type edge struct{ dependent, source int }
	seen := make(map[edge]struct{})
	dependencies := make([]PrepDependency, 0)
	for i := range plan.Meals {
		components := plan.Meals[i].Components
		for _, dependent := range components {
			if !validIndex(dependent.PlanRecipeIndex, assembled) || !assembled[dependent.PlanRecipeIndex] {
				continue
			}
			for _, source := range components {
				index := source.PlanRecipeIndex
				if !validIndex(index, assembled) || !known[index] || assembled[index] {
					continue
				}
				key := edge{dependent: dependent.PlanRecipeIndex, source: index}
				if _, exists := seen[key]; exists {
					continue
				}
				seen[key] = struct{}{}
				// No note: the edge itself says what feeds what, and a stored
				// note could only name a slice position, not a plan recipe id.
				dependencies = append(dependencies, PrepDependency{
					DependentRecipeIndex: key.dependent,
					SourceRecipeIndex:    key.source,
				})
			}
		}
	}
	return dependencies
}

func validIndex(index int, recipes []bool) bool {
	return index >= 0 && index < len(recipes)
}
