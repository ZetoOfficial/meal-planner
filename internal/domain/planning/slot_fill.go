package planning

import (
	"github.com/ZetoOfficial/meal-planner/internal/domain/catalog"
	"github.com/ZetoOfficial/meal-planner/internal/domain/recipe"
)

type SlotFill struct {
	SlotID           recipe.SlotID
	ProductID        catalog.ProductID
	PlannedInputQtyG float64
}
