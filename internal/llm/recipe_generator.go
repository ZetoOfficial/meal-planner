package llm

import (
	"context"

	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

// RecipeGenerator — адаптер LLM под usecase-порт planning.RecipeGenerator.
// Возвращает ТОЛЬКО draft-кандидатов; валидация и решение — в Go (ARCHITECTURE §8).
type RecipeGenerator struct {
	client Client
}

// NewRecipeGenerator оборачивает LLM-клиент.
func NewRecipeGenerator(c Client) *RecipeGenerator { return &RecipeGenerator{client: c} }

// Generate — TODO: построить промпт, вызвать client, распарсить structured output,
// вернуть drafts. Валидатор/solver разбираются с ними дальше.
func (g *RecipeGenerator) Generate(_ context.Context, _ any) ([]recipedom.TemplateDraft, error) {
	return nil, ErrNotImplemented
}
