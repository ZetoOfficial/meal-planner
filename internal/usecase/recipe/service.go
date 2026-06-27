package recipe

// Service — use cases рецептов (CreateRecipe/PromoteRecipe/...). TODO.
type Service struct {
	recipes RecipeRepository
}

// NewService собирает сервис рецептов.
func NewService(recipes RecipeRepository) *Service { return &Service{recipes: recipes} }
