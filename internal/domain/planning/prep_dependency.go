package planning

// PrepDependency records that one cooked batch feeds another: the dependent
// recipe uses part of the source recipe's output. Swapping the source breaks
// the dependent dish, so the graph must be visible before a swap is applied.
type PrepDependency struct {
	ID                int64
	DependentRecipeID PlanRecipeID
	SourceRecipeID    PlanRecipeID
	Note              *string
}
