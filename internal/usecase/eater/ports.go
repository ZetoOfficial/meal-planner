package eater

import (
	"context"

	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
)

type EaterRepository interface {
	Get(ctx context.Context, id eaterdom.ID) (eaterdom.Eater, error)
	CurrentGoal(ctx context.Context, id eaterdom.ID) (eaterdom.MacroGoal, error)
	SaveGoalVersion(ctx context.Context, g eaterdom.MacroGoal) error
}
