// Package solver — детерминированный построитель плана из кандидатов.
// Зависит только от доменных типов, НЕ от инфраструктуры (ARCHITECTURE §5.7).
// Целевая функция лексикографическая: hard feasibility → macro → strongest quality
// → rolling weekly → cost/reuse/storage/novelty → coverage.
package solver

import (
	"context"

	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	catalogdom "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	recipedom "github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

// Input — вход солвера.
type Input struct {
	Eaters   []eaterdom.Eater
	Products []catalogdom.Product
	Recipes  []recipedom.Template
	Novelty  int
}

// Planner строит план из доступных кандидатов.
type Planner struct{}

// New создаёт планировщик.
func New() *Planner { return &Planner{} }

// BuildPlan — TODO: реализация по ARCHITECTURE §5.7.
func (p *Planner) BuildPlan(_ context.Context, _ Input) (Result, error) { return Result{}, nil }
