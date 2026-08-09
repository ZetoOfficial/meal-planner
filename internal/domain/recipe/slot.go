package recipe

type SlotID int64

type Slot struct {
	ID                SlotID
	RoleTagCode       string
	Label             string
	QtyG              float64
	PreparedQtyFactor *float64
	Position          int
	Optional          bool
}
