package recipe

// Slot — слот шаблона: ловит класс по role_tag, заполняется продуктом в плане.
type Slot struct {
	RoleTagCode string
	Label       string
	Qty         float64
	Unit        string
}
