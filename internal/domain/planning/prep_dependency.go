package planning

// PrepDependency — блюдо B использует результат блюда A («вчерашняя курица»).
// Своп источника делает зависимое блюдо невалидным (ARCHITECTURE §5.4).
type PrepDependency struct {
	DependentRecipeID int64
	SourceRecipeID    int64
	Note              string
}
