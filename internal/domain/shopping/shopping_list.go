// Package shopping — список покупок как производная от плана: агрегирует slot_fill
// по продукту, считает количество/стоимость; перегенерация без потери отметок (ARCHITECTURE §5.8).
package shopping

// List — список покупок плана.
type List struct {
	PlanID int64
	Items  []Item
}

// Item — позиция списка покупок.
type Item struct {
	ProductID  int64
	TotalQtyG  float64
	Checked    bool
}
