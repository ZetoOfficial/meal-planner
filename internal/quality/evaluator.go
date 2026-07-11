package quality

import (
	"context"

	"github.com/ZetoOfficial/meal-planner/internal/domain/nutrition"
)

type Evaluator struct {
	rules Ruleset
}

func NewEvaluator(rules Ruleset) *Evaluator { return &Evaluator{rules: rules} }

func (e *Evaluator) EvaluatePlan(_ context.Context, _ any) []nutrition.RuleEvaluationResult {
	return nil
}
