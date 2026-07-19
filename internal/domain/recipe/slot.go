package recipe

type SlotID int64

type Slot struct {
	ID          SlotID
	RoleTagCode string
	Label       string
	Qty         float64
	Unit        string
	Position    int
	Optional    bool
}
