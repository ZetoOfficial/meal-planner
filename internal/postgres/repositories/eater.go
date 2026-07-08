// Package repositories — адаптеры Postgres: маппинг sqlc rows ↔ domain.
// sqlc-типы НЕ должны протекать в domain/usecase/solver/quality (ARCHITECTURE §10).
package repositories

import (
	"context"

	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
)

// EaterRepository реализует соответствующий usecase-порт поверх sqlc.
// Методы оставлены заглушками, потому что planning.Repositories требует этот порт.
type EaterRepository struct{}

func NewEaterRepository() *EaterRepository {
	return &EaterRepository{}
}

func (r *EaterRepository) Get(context.Context, eaterdom.ID) (eaterdom.Eater, error) {
	return eaterdom.Eater{}, ErrNotImplemented
}

func (r *EaterRepository) CurrentGoal(context.Context, eaterdom.ID) (eaterdom.MacroGoal, error) {
	return eaterdom.MacroGoal{}, ErrNotImplemented
}

func (r *EaterRepository) SaveGoalVersion(context.Context, eaterdom.MacroGoal) error {
	return ErrNotImplemented
}
