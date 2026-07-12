package shopping

type List struct {
	PlanID int64
	Items  []Item
}

type Item struct {
	ProductID int64
	TotalQtyG float64
	Checked   bool
}
