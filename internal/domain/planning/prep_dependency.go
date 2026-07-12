package planning

type PrepDependency struct {
	DependentRecipeID int64
	SourceRecipeID    int64
	Note              string
}
