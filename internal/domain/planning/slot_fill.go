package planning

import "github.com/ZetoOfficial/meal-planner/internal/domain/catalog"

type SlotFill struct {
	SlotID    int64
	ProductID catalog.ProductID
	QtyG      float64
}
