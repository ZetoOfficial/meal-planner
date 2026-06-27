package planning

import "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"

// SlotFill — какой продукт встал в слот экземпляра рецепта. Своп = смена ProductID.
type SlotFill struct {
	SlotID    int64
	ProductID catalog.ProductID
	QtyG      float64
}
