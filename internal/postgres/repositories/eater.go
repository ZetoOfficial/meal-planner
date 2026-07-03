// Package repositories — адаптеры Postgres: маппинг sqlc rows ↔ domain.
// sqlc-типы НЕ должны протекать в domain/usecase/solver/quality (ARCHITECTURE §10).
package repositories

import (
	"context"

	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
	"github.com/ZetoOfficial/meal-planner/internal/postgres/sqlc"
)

// EaterRepository реализует соответствующий usecase-порт поверх sqlc.
type EaterRepository struct {
	q sqlc.Querier
}

func NewEaterRepository(q sqlc.Querier) *EaterRepository {
	return &EaterRepository{q: q}
}

func (r *EaterRepository) Get(ctx context.Context, id eaterdom.ID) (eaterdom.Eater, error) {
	row, err := r.q.GetEater(ctx, int64(id))
	if err != nil {
		return eaterdom.Eater{}, err
	}
	return eaterdom.Eater{
		ID:   eaterdom.ID(row.ID),
		Name: row.Name,
		Sex:  eaterdom.Sex(row.Sex),
	}, nil
}

func (r *EaterRepository) CurrentGoal(ctx context.Context, id eaterdom.ID) (eaterdom.MacroGoal, error) {
	row, err := r.q.GetCurrentMacroGoal(ctx, int64(id))
	if err != nil {
		return eaterdom.MacroGoal{}, err
	}
	proteinMin, err := numericFloat64(row.ProteinMinG)
	if err != nil {
		return eaterdom.MacroGoal{}, err
	}
	proteinMax, err := numericFloat64(row.ProteinMaxG)
	if err != nil {
		return eaterdom.MacroGoal{}, err
	}
	fatMin, err := numericFloat64(row.FatMinG)
	if err != nil {
		return eaterdom.MacroGoal{}, err
	}
	fatMax, err := numericFloat64(row.FatMaxG)
	if err != nil {
		return eaterdom.MacroGoal{}, err
	}
	carbMin, err := numericFloat64(row.CarbMinG)
	if err != nil {
		return eaterdom.MacroGoal{}, err
	}
	carbMax, err := numericFloat64(row.CarbMaxG)
	if err != nil {
		return eaterdom.MacroGoal{}, err
	}
	return eaterdom.MacroGoal{
		EaterID:       eaterdom.ID(row.EaterID),
		EffectiveFrom: row.EffectiveFrom.Time,
		Range: eaterdom.MacroRange{
			ProteinMinG: proteinMin,
			ProteinMaxG: proteinMax,
			FatMinG:     fatMin,
			FatMaxG:     fatMax,
			CarbMinG:    carbMin,
			CarbMaxG:    carbMax,
			KcalMin:     int(row.KcalMin),
			KcalMax:     int(row.KcalMax),
		},
	}, nil
}

func (r *EaterRepository) SaveGoalVersion(context.Context, eaterdom.MacroGoal) error {
	return ErrNotImplemented
}
