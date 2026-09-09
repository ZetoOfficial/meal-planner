package planning

import (
	"github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	"github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

type SlotFillID int64

type SlotFill struct {
	ID               SlotFillID
	SlotID           recipe.SlotID
	ProductID        catalog.ProductID
	PlannedInputQtyG float64
}
