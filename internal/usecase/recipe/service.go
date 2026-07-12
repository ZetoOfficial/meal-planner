package recipe

type Service struct {
	recipes RecipeRepository
}

func NewService(recipes RecipeRepository) *Service { return &Service{recipes: recipes} }
