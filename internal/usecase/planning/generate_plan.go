package planning

import "context"

type GeneratePlan struct {
	plans PlanRepository
	tx    TxManager
}

func NewGeneratePlan(plans PlanRepository, tx TxManager) *GeneratePlan {
	return &GeneratePlan{plans: plans, tx: tx}
}

func (uc *GeneratePlan) Handle(_ context.Context, _ any) (any, error) { return nil, nil }
