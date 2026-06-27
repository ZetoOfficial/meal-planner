// Package planning (usecase) — генерация плана, preview/apply свопа.
// Оркестрирует solver, quality evaluator и LLM recipe generator через порты.
package planning

import (
	"context"

	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

// PlanRepository — порт хранения планов.
type PlanRepository interface {
	Save(ctx context.Context, p any) error
	Get(ctx context.Context, id int64) (any, error)
}

// RecipeGenerator — порт LLM-генератора кандидатов (реализуется llm-адаптером).
// LLM возвращает только draft-кандидатов; валидация/решение — в Go (ARCHITECTURE §8).
type RecipeGenerator interface {
	Generate(ctx context.Context, input any) ([]recipedom.TemplateDraft, error)
}

// TxManager — транзакционная граница на уровне use case (ARCHITECTURE §11).
type TxManager interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}
