// Package eater (usecase) — сценарии профиля едока, целей и fixed meals.
package eater

import (
	"context"

	eaterdom "github.com/ZetoOfficial/meal-planner/internal/domain/eater"
)

// EaterRepository — порт доступа к едокам и их версионируемым целям.
type EaterRepository interface {
	Get(ctx context.Context, id eaterdom.ID) (eaterdom.Eater, error)
	CurrentGoal(ctx context.Context, id eaterdom.ID) (eaterdom.MacroGoal, error)
	SaveGoalVersion(ctx context.Context, g eaterdom.MacroGoal) error
}
