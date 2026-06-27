package planning

import "context"

// GeneratePlan — use case reuse-first генерации (ARCHITECTURE §11):
// permanent-кандидаты → при нехватке/novelty≥2 LLM → валидатор → solver → quality.
type GeneratePlan struct {
	plans PlanRepository
	tx    TxManager
}

// NewGeneratePlan собирает use case.
func NewGeneratePlan(plans PlanRepository, tx TxManager) *GeneratePlan {
	return &GeneratePlan{plans: plans, tx: tx}
}

// Handle — TODO: реализация по ARCHITECTURE §11.
func (uc *GeneratePlan) Handle(_ context.Context, _ any) (any, error) { return nil, nil }
