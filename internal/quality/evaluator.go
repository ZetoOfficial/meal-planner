// Package quality — детерминированный evaluator правил качества рациона.
// Правила в MVP живут в коде/конфиге; в БД — profile binding, overrides, summaries,
// результаты (ARCHITECTURE §5.6). Статусы: pass/soft_fail/warn/unknown.
package quality

import (
	"context"

	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
)

// Evaluator оценивает план/день/окно против ruleset.
type Evaluator struct {
	rules Ruleset
}

// NewEvaluator собирает evaluator с набором правил.
func NewEvaluator(rules Ruleset) *Evaluator { return &Evaluator{rules: rules} }

// EvaluatePlan — TODO: вернуть набор RuleEvaluationResult + score.
func (e *Evaluator) EvaluatePlan(_ context.Context, _ any) []nutrition.RuleEvaluationResult {
	return nil
}
